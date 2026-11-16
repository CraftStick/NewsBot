package digest

import (
	"testing"

	"treesheild-newsbot/internal/news"
)

// В истории ссылка приходит из HTML (&amp;), в ленте — сырая: раньше не совпадали.
func TestDropPublishedLinkWithEscapedQuery(t *testing.T) {
	history := []publishedItem{{Title: "Другой заголовок", Link: "https://habr.com/ru/articles/1/?utm_campaign=1&amp;utm_source=habrahabr"}}
	articles := []news.Article{{Title: "Совсем иначе", Link: "https://habr.com/ru/articles/1/?utm_campaign=1&utm_source=habrahabr"}}
	if got := DropPublished(articles, history); len(got) != 0 {
		t.Fatalf("статья уже публиковалась, но осталась: %+v", got)
	}
	articles[0].Link = "https://habr.com/ru/articles/1/?utm_source=other"
	if got := DropPublished(articles, history); len(got) != 0 {
		t.Fatal("разные utm не должны делать ссылку новой")
	}
}
