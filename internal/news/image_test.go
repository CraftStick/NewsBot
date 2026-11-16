package news

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestItemImage(t *testing.T) {
	it := &gofeed.Item{
		Image:      &gofeed.Image{URL: "https://kod.ru/favicon.png"},
		Enclosures: []*gofeed.Enclosure{{URL: "https://api.kod.ru/c.png"}},
	}
	if got := itemImage(it); got != "https://api.kod.ru/c.png" {
		t.Fatalf("got %q", got)
	}
	it.Enclosures = nil
	if got := itemImage(it); got != "" {
		t.Fatalf("favicon не должен проходить: %q", got)
	}
}
