package digest

import (
	"encoding/json"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"treesheild-newsbot/internal/news"
)

const (
	// historyWindow — сколько помним уже опубликованные темы. Лента берётся за
	// 7 дней, но сюжет («Яндекс» запустил оператора) тянется по лентам дольше и
	// иначе попадает в дайджест повторно. Месяц закрывает 4 выпуска.
	historyWindow = 28 * 24 * time.Hour
	// historyMaxItems — потолок на размер файла: 10 выпусков по 6 пунктов.
	historyMaxItems = 60
	// historyPromptItems — сколько последних тем перечислять модели.
	historyPromptItems = 24
)

// publishedItem — тема, уже ушедшая в дайджест. Ссылка — надёжный идентификатор
// новости; заголовок нужен модели, чтобы отсечь тот же сюжет из другой ленты.
type publishedItem struct {
	Title  string    `json:"title"`
	Link   string    `json:"link"`
	SentAt time.Time `json:"sent_at"`
}

func historyFilePath() string {
	if p := strings.TrimSpace(os.Getenv("HISTORY_FILE")); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), ".digest_history")
	}
	return ".digest_history"
}

// ReadHistory возвращает недавно опубликованные темы, свежие — в конце.
// Отсутствующий или битый файл не ошибка: дайджест важнее памяти, просто
// соберётся без дедупа.
func ReadHistory(now time.Time) []publishedItem {
	raw, err := os.ReadFile(historyFilePath())
	if err != nil {
		return nil
	}
	var items []publishedItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return pruneHistory(items, now)
}

func pruneHistory(items []publishedItem, now time.Time) []publishedItem {
	out := make([]publishedItem, 0, len(items))
	for _, it := range items {
		if now.Sub(it.SentAt) <= historyWindow {
			out = append(out, it)
		}
	}
	if len(out) > historyMaxItems {
		out = out[len(out)-historyMaxItems:]
	}
	return out
}

// RecordHistory дописывает темы отправленного дайджеста (temp+rename, как
// и отметка времени в internal/app/state.go).
func RecordHistory(newsHTML string, now time.Time) error {
	added := extractPublishedItems(newsHTML, now)
	items := pruneHistory(append(ReadHistory(now), added...), now)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	path := historyFilePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// extractPublishedItems достаёт темы из готового HTML дайджеста — уже со
// ссылками, проставленными ensureNewsLinks.
func extractPublishedItems(newsHTML string, now time.Time) []publishedItem {
	var out []publishedItem
	for _, block := range splitNewsBlocks(newsHTML) {
		title := ExtractNewsTitle(block)
		link := extractNewsURL(block)
		if title == "" && link == "" {
			continue
		}
		out = append(out, publishedItem{Title: title, Link: link, SentAt: now})
	}
	if len(out) == 0 { // одиночный мини-пост: без нумерации, заголовок — первый <b>
		link := extractNewsURL(newsHTML)
		title := ""
		if m := boldRE.FindStringSubmatch(newsHTML); m != nil {
			title = strings.TrimSpace(news.StripHTML(m[1]))
		}
		if title != "" || link != "" {
			out = append(out, publishedItem{Title: title, Link: link, SentAt: now})
		}
	}
	return out
}

var boldRE = regexp.MustCompile(`(?is)<b>(.*?)</b>`)

// DropPublished убирает из ленты статьи, которые уже уходили в дайджест. Ссылка
// совпадает точно; заголовок сравниваем нормализованным — на случай, если та же
// новость пришла из другой ленты.
func DropPublished(articles []news.Article, history []publishedItem) []news.Article {
	if len(history) == 0 {
		return articles
	}
	links := make(map[string]bool, len(history))
	titles := make(map[string]bool, len(history))
	seen := make([]map[string]bool, 0, len(history))
	for _, it := range history {
		if l := normalizeLink(it.Link); l != "" {
			links[l] = true
		}
		if t := normalizeTitle(it.Title); t != "" {
			titles[t] = true
			seen = append(seen, titleStems(t))
		}
	}

	out := make([]news.Article, 0, len(articles))
	for _, a := range articles {
		if links[normalizeLink(a.Link)] || titles[normalizeTitle(a.Title)] || similarToAny(a.Title, seen) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// HistoryTitles — темы для промпта, свежие первыми.
func HistoryTitles(history []publishedItem) []string {
	out := make([]string, 0, historyPromptItems)
	for i := len(history) - 1; i >= 0 && len(out) < historyPromptItems; i-- {
		if t := strings.TrimSpace(history[i].Title); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizeLink приводит ссылку к виду для сравнения: из HTML дайджеста она
// приходит с &amp;, а utm-метки у одной статьи разные.
func normalizeLink(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(k, "utm_") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

// titleStems — основы значимых слов заголовка (первые 5 букв слов от 4 букв).
func titleStems(normalized string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(normalized) {
		if r := []rune(w); len(r) >= 4 {
			if len(r) > 5 {
				r = r[:5]
			}
			out[string(r)] = true
		}
	}
	return out
}

// similarToAny — тот же сюжет из другой ленты: общих основ ≥3 и ≥60% меньшего заголовка.
func similarToAny(title string, seen []map[string]bool) bool {
	a := titleStems(normalizeTitle(title))
	for _, b := range seen {
		common := 0
		for k := range a {
			if b[k] {
				common++
			}
		}
		min := len(a)
		if len(b) < min {
			min = len(b)
		}
		if common >= 3 && common*10 >= min*6 {
			return true
		}
	}
	return false
}
