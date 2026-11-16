package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"treesheild-newsbot/internal/news"
)

func TestSaveMarksPublished(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.db")
	err := Save(p, Run{
		StartedAt: time.Now(), DigestHTML: `<a href="https://a/1?x=1&amp;y=2">t</a>`,
		Candidates: []news.Article{{Title: "A", Link: "https://a/1?x=1&y=2"}, {Title: "B", Link: "https://b/2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", p)
	defer func() { _ = db.Close() }()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM candidates WHERE published=1`).Scan(&n)
	if n != 1 {
		t.Fatalf("published=%d, ждали 1", n)
	}
}
