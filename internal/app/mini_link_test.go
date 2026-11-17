package app

import (
	"strings"
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestMiniHTMLOnlySourceIsLink(t *testing.T) {
	for _, sum := range []string{"", "Описание"} {
		got := miniHTML(news.Article{Source: "Gram News", Title: "Текст", Summary: sum, Link: "https://x/1"})
		if strings.Count(got, "<a ") != 1 || !strings.Contains(got, `<a href="https://x/1"><i>Gram News</i></a>`) {
			t.Fatal(got)
		}
	}
}
