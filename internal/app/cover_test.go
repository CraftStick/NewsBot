package app

import (
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestKodCandidates(t *testing.T) {
	arts := []news.Article{
		{Link: "https://other.ru/a"},
		{Link: "https://kod.ru/old"},
		{Link: "https://kod.ru/in/"},
	}
	got := kodCandidates(`<a href="https://kod.ru/in">x</a>`, arts)
	if len(got) != 2 || got[0].Link != "https://kod.ru/in/" || got[1].Link != "https://kod.ru/old" {
		t.Fatalf("порядок: %v", got)
	}
	if len(kodCandidates("", []news.Article{{Link: "https://other.ru/a"}})) != 0 {
		t.Fatal("чужой источник попал в обложку")
	}
}
