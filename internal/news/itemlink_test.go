package news

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestItemLink(t *testing.T) {
	for _, c := range []struct {
		it   gofeed.Item
		want string
	}{
		{gofeed.Item{Link: "https://a/1", GUID: "https://a/2"}, "https://a/1"},
		{gofeed.Item{GUID: "https://a/2"}, "https://a/2"},
		{gofeed.Item{GUID: "id-17"}, ""},
	} {
		if got := itemLink(&c.it); got != c.want {
			t.Errorf("%+v: %q, ждали %q", c.it, got, c.want)
		}
	}
}
