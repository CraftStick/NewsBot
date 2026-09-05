package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDropPublishedByLink(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	history := []publishedItem{
		{Title: "«Яндекс» запустил виртуального оператора", Link: "https://cnews.ru/yandex-sim", SentAt: now.Add(-24 * time.Hour)},
	}
	articles := []Article{
		{Title: "Яндекс Сим: тарифы", Link: "https://cnews.ru/yandex-sim"},
		{Title: "«Яндекс» запустил виртуального оператора", Link: "https://ixbt.com/other"}, // тот же сюжет из другой ленты
		{Title: "Минцифры и реформа лицензий", Link: "https://comnews.ru/reform"},
	}

	got := dropPublished(articles, history)
	if len(got) != 1 || got[0].Link != "https://comnews.ru/reform" {
		t.Fatalf("ожидали только новую статью, получили %+v", got)
	}
	if len(dropPublished(articles, nil)) != len(articles) {
		t.Fatal("пустая история не должна ничего отбрасывать")
	}
}

func TestHistoryPrunesOldItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	items := []publishedItem{
		{Title: "старое", Link: "https://a", SentAt: now.Add(-historyWindow - time.Hour)},
		{Title: "свежее", Link: "https://b", SentAt: now.Add(-historyWindow + time.Hour)},
	}

	got := pruneHistory(items, now)
	if len(got) != 1 || got[0].Title != "свежее" {
		t.Fatalf("окно памяти не соблюдено: %+v", got)
	}
}

// Темы для промпта — свежие первыми и не больше лимита.
func TestHistoryTitlesNewestFirst(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	var items []publishedItem
	for i := 0; i < historyPromptItems+5; i++ {
		items = append(items, publishedItem{Title: string(rune('а' + i%32)), Link: "https://x", SentAt: now})
	}
	items = append(items, publishedItem{Title: "самая свежая", Link: "https://y", SentAt: now})

	got := historyTitles(items)
	if len(got) != historyPromptItems {
		t.Fatalf("ожидали %d тем, получили %d", historyPromptItems, len(got))
	}
	if got[0] != "самая свежая" {
		t.Fatalf("порядок нарушен, первой должна быть свежая тема: %q", got[0])
	}
}

// Полный круг: дайджест → файл → фильтр ленты на следующей неделе.
func TestHistoryRoundTrip(t *testing.T) {
	t.Setenv("HISTORY_FILE", filepath.Join(t.TempDir(), ".digest_history"))
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	body := makeBody([][2]string{
		{"«Яндекс» запустил оператора", "Первое предложение. Второе предложение."},
		{"Реформа Минцифры", "Первое предложение. Второе предложение."},
	})
	if err := recordDigestHistory(body, now); err != nil {
		t.Fatalf("запись истории: %v", err)
	}

	got := readDigestHistory(now)
	if len(got) != 2 {
		t.Fatalf("ожидали 2 темы, получили %d (%+v)", len(got), got)
	}
	if got[0].Link != "https://example.com/1" {
		t.Fatalf("ссылка не извлеклась: %+v", got[0])
	}

	// Через неделю та же новость в ленте — не должна пройти в промпт.
	next := []Article{{Title: "Яндекс Сим", Link: "https://example.com/1"}}
	if left := dropPublished(next, got); len(left) != 0 {
		t.Fatalf("повтор не отсеян: %+v", left)
	}
}
