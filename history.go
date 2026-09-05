package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// readDigestHistory возвращает недавно опубликованные темы, свежие — в конце.
// Отсутствующий или битый файл не ошибка: дайджест важнее памяти, просто
// соберётся без дедупа.
func readDigestHistory(now time.Time) []publishedItem {
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

// recordDigestHistory дописывает темы отправленного дайджеста (temp+rename, как
// и отметка времени в state.go).
func recordDigestHistory(newsHTML string, now time.Time) error {
	items := pruneHistory(append(readDigestHistory(now), extractPublishedItems(newsHTML, now)...), now)
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
		title := extractNewsTitle(block)
		link := extractNewsURL(block)
		if title == "" && link == "" {
			continue
		}
		out = append(out, publishedItem{Title: title, Link: link, SentAt: now})
	}
	return out
}

// dropPublished убирает из ленты статьи, которые уже уходили в дайджест. Ссылка
// совпадает точно; заголовок сравниваем нормализованным — на случай, если та же
// новость пришла из другой ленты.
func dropPublished(articles []Article, history []publishedItem) []Article {
	if len(history) == 0 {
		return articles
	}
	links := make(map[string]bool, len(history))
	titles := make(map[string]bool, len(history))
	for _, it := range history {
		if l := strings.TrimSpace(it.Link); l != "" {
			links[l] = true
		}
		if t := normalizeTitle(it.Title); t != "" {
			titles[t] = true
		}
	}

	out := make([]Article, 0, len(articles))
	for _, a := range articles {
		if links[strings.TrimSpace(a.Link)] || titles[normalizeTitle(a.Title)] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// historyTitles — темы для промпта, свежие первыми.
func historyTitles(history []publishedItem) []string {
	out := make([]string, 0, historyPromptItems)
	for i := len(history) - 1; i >= 0 && len(out) < historyPromptItems; i-- {
		if t := strings.TrimSpace(history[i].Title); t != "" {
			out = append(out, t)
		}
	}
	return out
}
