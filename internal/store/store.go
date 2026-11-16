// Package store — журнал запусков дайджеста в SQLite для разбора ошибок:
// что вышло, из каких кандидатов выбирали, чем закончился запуск.
package store

import (
	"database/sql"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // чистый Go, без CGO — кросс-сборка не ломается

	"treesheild-newsbot/internal/news"
)

// Run — итог одного запуска RunDigest.
type Run struct {
	StartedAt  time.Time
	Duration   time.Duration
	Err        string // пусто = успех
	FeedsFail  []string
	Candidates []news.Article // что дошло до Gemini после отсева дублей
	DigestHTML string         // пусто, если дайджест не собран
}

const schema = `
CREATE TABLE IF NOT EXISTS runs(
  id INTEGER PRIMARY KEY, started_at TEXT, duration_s REAL, error TEXT,
  feeds_failed TEXT, candidates INTEGER, digest_html TEXT);
CREATE TABLE IF NOT EXISTS candidates(
  run_id INTEGER REFERENCES runs(id), source TEXT, title TEXT, link TEXT,
  relevance INTEGER, published INTEGER);
CREATE INDEX IF NOT EXISTS candidates_run ON candidates(run_id);`

// Path — файл БД: DIGEST_DB или рядом с бинарником.
func Path() string {
	if p := strings.TrimSpace(os.Getenv("DIGEST_DB")); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "digest.db")
	}
	return "digest.db"
}

// Save пишет запуск в БД по пути path. Ошибку вызывающий только логирует:
// журнал не должен ронять дайджест.
func Save(path string, r Run) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`INSERT INTO runs(started_at,duration_s,error,feeds_failed,candidates,digest_html) VALUES(?,?,?,?,?,?)`,
		r.StartedAt.Format(time.RFC3339), r.Duration.Seconds(), r.Err,
		strings.Join(r.FeedsFail, ", "), len(r.Candidates), r.DigestHTML)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	text := html.UnescapeString(r.DigestHTML)
	for _, a := range r.Candidates {
		pub := a.Link != "" && strings.Contains(text, html.UnescapeString(a.Link))
		if _, err := tx.Exec(`INSERT INTO candidates VALUES(?,?,?,?,?,?)`,
			id, a.Source, a.Title, a.Link, a.Relevance, pub); err != nil {
			return err
		}
	}
	return tx.Commit()
}
