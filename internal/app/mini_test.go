package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"treesheild-newsbot/internal/news"
)

func TestNewMini(t *testing.T) {
	now := time.Now()
	arts := []news.Article{{Link: "c", PublishedAt: now}, {Link: "b", PublishedAt: now}, {Link: "a", PublishedAt: now}, {Link: "old", PublishedAt: now.Add(-9 * time.Hour)}}
	since := now.Add(-time.Hour)
	got := newMini(arts, []string{"a"}, 5, since)
	if len(got) != 2 || got[0].Link != "b" || got[1].Link != "c" {
		t.Fatalf("старые первыми, без виденных: %+v", got)
	}
	if got := newMini(arts, nil, 2, since); len(got) != 2 || got[0].Link != "a" {
		t.Fatalf("лимит берёт самые старые: %+v", got)
	}
}

func TestSeenRoundTripAndHTML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "seen")
	if _, ok := readSeen(p); ok {
		t.Fatal("файла нет — ok должен быть false")
	}
	if err := writeSeen(p, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if l, ok := readSeen(p); !ok || len(l) != 1 {
		t.Fatalf("%v %v", l, ok)
	}
	h := miniHTML(news.Article{Title: "A <b> & B", Link: "http://x/?a=1&b=2", Summary: "s"})
	if !strings.Contains(h, "A &lt;b&gt; &amp; B") || !strings.Contains(h, "a=1&amp;b=2") {
		t.Fatal(h)
	}
}

func TestMiniHTMLFitsCaption(t *testing.T) {
	a := news.Article{Title: "T", Link: "http://x", Source: "S", Summary: strings.Repeat("слово ", 400)}
	if n := len([]rune(miniHTML(a))); n > captionMax {
		t.Fatalf("подпись %d > %d", n, captionMax)
	}
	short := miniHTML(news.Article{Title: "T", Link: "http://x", Source: "S", Summary: "коротко"})
	if !strings.Contains(short, "коротко") || strings.Contains(short, "…") {
		t.Fatal(short)
	}
}
