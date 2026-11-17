package app

import (
	"strings"
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestMiniHTMLLinkOnlyWithSummary(t *testing.T) {
	a := news.Article{Source: "Gram News", Title: "Текст поста", Link: "https://x/1"}
	got := miniHTML(a)
	if strings.Contains(got, `<b><a`) || !strings.Contains(got, `<a href="https://x/1"><i>Gram News</i></a>`) {
		t.Fatal(got)
	}
	a.Summary = "Описание"
	if !strings.Contains(miniHTML(a), `<b><a href="https://x/1">Текст поста</a></b>`) {
		t.Fatal("заголовок с описанием должен быть ссылкой")
	}
}
