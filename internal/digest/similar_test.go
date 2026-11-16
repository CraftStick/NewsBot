package digest

import (
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestDropPublishedSameStoryOtherTitle(t *testing.T) {
	hist := []publishedItem{{Title: "Beget восстановил работу сайтов после масштабного сбоя", Link: "https://a/1"}}
	arts := []news.Article{
		{Title: "Хостинг Beget после масштабного сбоя восстановил сайты", Link: "https://b/2"},
		{Title: "Роскомнадзор заблокировал мессенджер в России", Link: "https://c/3"},
	}
	got := DropPublished(arts, hist)
	if len(got) != 1 || got[0].Link != "https://c/3" {
		t.Fatalf("осталось %+v", got)
	}
}
