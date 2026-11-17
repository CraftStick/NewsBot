package app

import (
	"path/filepath"
	"testing"
	"time"

	"treesheild-newsbot/internal/digest"
	"treesheild-newsbot/internal/news"
)

// опубликованный мини-пост должен попадать в историю дайджеста и отсекаться при сборке выпуска
func TestMiniPostGoesToDigestHistory(t *testing.T) {
	t.Setenv("HISTORY_FILE", filepath.Join(t.TempDir(), "h"))
	a := news.Article{Title: "Telegram ответил на блокировки в Иордании и Ираке", Summary: "Коротко о событии.", Link: "https://kod.ru/telegram-jordan", Source: "Код Дурова"}
	now := time.Now()
	if err := digest.RecordHistory(miniHTML(a), now); err != nil {
		t.Fatal(err)
	}
	if got := digest.DropPublished([]news.Article{a}, digest.ReadHistory(now)); len(got) != 0 {
		t.Fatalf("мини-пост не попал в историю: %v", got)
	}
}
