package app

import (
	"strings"
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestBatchHTML(t *testing.T) {
	var arts []news.Article
	for i := 0; i < 200; i++ {
		arts = append(arts, news.Article{Title: strings.Repeat("я", 300), Link: "https://x/1"})
	}
	if n := len([]rune(batchHTML(arts))); n > 4096 {
		t.Fatalf("подборка %d знаков, лимит 4096", n)
	}
}
