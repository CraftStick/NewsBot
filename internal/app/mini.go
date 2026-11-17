package app

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"treesheild-newsbot/internal/config"
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

// miniHTML — заголовок-ссылка, описание и источник; целиком укладывается в подпись к фото.
func miniHTML(a news.Article) string {
	sum := []rune(a.Summary)
	for {
		s := fmt.Sprintf(`<b><a href="%s">%s</a></b>`, html.EscapeString(a.Link), html.EscapeString(a.Title))
		if len(sum) > 0 {
			s += "\n\n" + html.EscapeString(string(sum))
		}
		s += "\n\n<i>" + html.EscapeString(a.Source) + "</i>"
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
	for _, a := range newMini(arts, seen, miniMaxPerTick, time.Now().Add(-miniMaxAge)) {
		photo := news.DownloadImage(ctx, news.ImageFor(ctx, a)) // nil — уйдёт текстом
		if err := telegram.SendMini(cfg, miniHTML(a), photo); err != nil {
			return err // не отмечена — уйдёт на следующем проходе
		}
		seen = append(seen, a.Link)
		if err := writeSeen(path, seen); err != nil {
			return err
		}
		log.Printf("мини-пост: %s", a.Title)
	}
	return nil
}
