package telegram

import (
	"strings"
	"testing"
)

func TestSkippedAndPendingText(t *testing.T) {
	if s := skippedText(); !strings.Contains(s, "Новых статей пока не было") {
		t.Fatalf("пустое состояние: %q", s)
	}
	NoteChecked()
	if s := skippedText(); !strings.Contains(s, "отсеивать было нечего") {
		t.Fatalf("ничего не отсеяно: %q", s)
	}
	NoteSkipped("дубль", "Заголовок")
	if s := skippedText(); !strings.Contains(s, "дубли 1") || !strings.Contains(s, "Заголовок") {
		t.Fatalf("счётчики: %q", s)
	}
	t.Chdir(t.TempDir()) // pendingDir относительный
	key, _ := savePending("<b>Тема поста</b>\nтекст", "", "")
	if ps := pendingList(); len(ps) != 1 || ps[0].key != key || ps[0].title != "Тема поста" {
		t.Fatalf("pendingList: %+v", ps)
	}
}
