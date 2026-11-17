package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
	"treesheild-newsbot/internal/gemini"
	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/telegram"
)

const (
	miniMaxPerTick = 5   // защита от потока, если kod.ru выложил пачку
	miniSeenMax    = 300 // сколько последних ссылок помним
	// Старше — не мини-пост: новая лента не завалит личку архивом.
	miniMaxAge = 3 * time.Hour
)

var miniMu sync.Mutex

// miniRecent — заголовки отправленных в этом запуске: убирает дубли kod.ru и Gram News.
var miniRecent []string

func recentDup(title string, recent []string) bool {
	for _, t := range recent {
		if digest.SimilarTitles(title, t) {
			return true
		}
	}
	return false
}

// miniSince — нижняя граница свежести. С 00 до 08 мини-посты копятся, утром (08–11)
// уходит всё накопленное за ночь.
func miniSince(now time.Time) time.Time {
	if os.Getenv("MINI_MAX_AGE") != "" {
		return now.Add(-maxAge())
	}
	if h := now.Hour(); h >= 8 && h < 11 {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	return now.Add(-maxAge())
}

func quietHours(now time.Time) bool {
	return os.Getenv("MINI_MAX_AGE") == "" && now.Hour() < 8
}

func miniSeenPath() string {
	if p := os.Getenv("MINI_SEEN_FILE"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), ".mini_seen")
	}
	return ".mini_seen"
}

// readSeen: ok=false — файла ещё нет (первый запуск).
func readSeen(path string) (links []string, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if json.Unmarshal(raw, &links) != nil {
		return nil, false
	}
	return links, true
}

func writeSeen(path string, links []string) error {
	if len(links) > miniSeenMax {
		links = links[len(links)-miniSeenMax:]
	}
	data, err := json.Marshal(links)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// captionMax — лимит подписи к фото в Telegram.
const captionMax = 1024

// miniHTML — жирный заголовок, описание и источник-ссылка; целиком укладывается в подпись к фото.
func miniHTML(a news.Article) string {
	sum := []rune(a.Summary)
	for {
		s := fmt.Sprintf(`%s  <b>%s</b>`, miniEmoji(a), html.EscapeString(a.Title))
		if len([]rune(a.Title)) > 200 { // длинный текст поста целиком жирным нечитаем
			s = fmt.Sprintf("%s  %s", miniEmoji(a), html.EscapeString(a.Title))
		}
		if len(sum) > 0 {
			s += "\n\n" + html.EscapeString(string(sum))
		}
		s += fmt.Sprintf("\n\n<a href=\"%s\"><i>%s</i></a>", html.EscapeString(a.Link), html.EscapeString(a.Source))
		if len([]rune(s)) <= captionMax || len(sum) == 0 {
			return s
		}
		sum = append(sum[:len(sum)*8/10-1], '…') // ужимаем описание, пока не влезет
	}
}

// newMini — непоказанные статьи, старые первыми, не больше max.
func newMini(arts []news.Article, seen []string, max int, since time.Time) []news.Article {
	have := make(map[string]struct{}, len(seen))
	for _, l := range seen {
		have[l] = struct{}{}
	}
	var out []news.Article
	for i := len(arts) - 1; i >= 0; i-- { // arts: новые первыми
		if _, ok := have[arts[i].Link]; !ok && arts[i].PublishedAt.After(since) {
			out = append(out, arts[i])
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// RunMini шлёт в личку новые записи kod.ru мини-постами. Первый запуск только
// запоминает текущую ленту, чтобы не завалить личку старьём.
func RunMini(cfg config.Config) (err error) {
	if !miniMu.TryLock() {
		return nil
	}
	defer miniMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("паника: %v", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	arts, err := news.FetchMini(ctx)
	if err != nil {
		return err
	}
	path := miniSeenPath()
	seen, ok := readSeen(path)
	if !ok {
		for _, a := range arts {
			seen = append(seen, a.Link)
		}
		log.Printf("мини-посты: первый запуск, запомнил %d записей", len(seen))
		return writeSeen(path, seen)
	}
	if quietHours(time.Now()) {
		return nil
	}
	for _, a := range newMini(arts, seen, miniMaxPerTick, miniSince(time.Now())) {
		if recentDup(a.Title, miniRecent) {
			seen = append(seen, a.Link) // тот же сюжет уже присылали из другого источника
			log.Printf("мини-пост: дубль, пропущен: %s", a.Title)
			continue
		}
		if a.Source == "Код Дурова" && !gemini.OnTopic(ctx, cfg, a.Title, a.Summary) {
			seen = append(seen, a.Link)
			log.Printf("мини-пост: не по теме (Gemini): %s", a.Title)
			if err := writeSeen(path, seen); err != nil { // иначе проверка темы (и квота Gemini) повторяется каждый тик
				return err
			}
			continue
		}
		photo := news.DownloadImage(ctx, news.ImageFor(ctx, a)) // nil — уйдёт текстом
		if err := telegram.SendMini(cfg, miniHTML(a), photo); err != nil {
			return err // не отмечена — уйдёт на следующем проходе
		}
		seen = append(seen, a.Link)
		if miniRecent = append(miniRecent, a.Title); len(miniRecent) > 50 {
			miniRecent = miniRecent[1:]
		}
		if err := writeSeen(path, seen); err != nil {
			return err
		}
		log.Printf("мини-пост: %s", a.Title)
	}
	return nil
}

// maxAge — окно свежести; MINI_MAX_AGE (например 48h) нужен для ручной проверки.
func maxAge() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("MINI_MAX_AGE")); err == nil && d > 0 {
		return d
	}
	return miniMaxAge
}

// miniEmoji — значок по теме новости; первое совпавшее правило, иначе 📰.
func miniEmoji(a news.Article) string {
	if a.EmojiID != "" {
		return tgEmoji(a.EmojiID, a.EmojiChar)
	}
	t := " " + strings.ToLower(a.Title+" "+a.Summary)
	rules := []struct {
		id, fallback string
		words        []string
	}{
		{"5348435100541404792", "⛔️", []string{"блокир", "роскомнадзор", "ркн", "запрет", "замедл", "впн", "vpn"}},
		{"5348230282140993341", "🛡️", []string{"утечк", "взлом", "хакер", "уязвим", "кибер", "вирус", "шифр", "приватн", "персональн", "мошенн", "фишинг"}},
		{"5348414471813483004", "☕️", []string{"закон", "суд", "штраф", "госдум", "регулир"}},
		{"5348482508390419372", "📱", []string{"telegram", "телеграм", "whatsapp", "мессенджер"}},
		{"5348302871383263062", "🧠", []string{" ии ", " ии-", " ии,", "нейросет", "искусственн", "openai", "chatgpt"}},
	}
	for _, r := range rules {
		for _, w := range r.words {
			if strings.Contains(t, w) {
				return tgEmoji(r.id, r.fallback)
			}
		}
	}
	return tgEmoji("5951891646445523477", "📰")
}

func tgEmoji(id, fallback string) string {
	return `<tg-emoji emoji-id="` + id + `">` + fallback + `</tg-emoji>`
}

// manualSent — что уже присылали по кнопке «Одиночный пост», чтобы не повторять одно и то же.
var manualSent = map[string]bool{}

// RunMiniOne шлёт самую свежую по теме новость, которую ещё не присылали по кнопке.
func RunMiniOne(cfg config.Config) error {
	miniMu.Lock()
	defer miniMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	arts, err := news.FetchMini(ctx)
	if err != nil {
		return err
	}
	for _, a := range arts { // новые первыми
		if manualSent[a.Link] || (a.Source == "Код Дурова" && !gemini.OnTopic(ctx, cfg, a.Title, a.Summary)) {
			continue
		}
		photo := news.DownloadImage(ctx, news.ImageFor(ctx, a))
		if err := telegram.SendMini(cfg, miniHTML(a), photo); err != nil {
			return err
		}
		manualSent[a.Link] = true
		// чтобы фоновая лента не прислала ту же новость второй раз
		path := miniSeenPath()
		if seen, ok := readSeen(path); ok {
			_ = writeSeen(path, append(seen, a.Link))
		}
		log.Printf("мини-пост по кнопке: %s", a.Title)
		return nil
	}
	return errors.New("новее подходящих новостей нет")
}

// ClearMini забывает просмотренные ссылки; следующий опрос заново снимет «базу» без отправки.
func ClearMini() error {
	miniMu.Lock()
	defer miniMu.Unlock()
	if err := os.Remove(miniSeenPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	miniRecent = nil
	manualSent = map[string]bool{}
	return nil
}
