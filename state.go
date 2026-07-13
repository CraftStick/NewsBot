package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Отметка времени последней успешной отправки дайджеста. Нужна для навёрстывания:
// после простоя (ребут в пятницу) бот при старте видит пропущенный слот и досылает.
func stateFilePath() string {
	if p := strings.TrimSpace(os.Getenv("STATE_FILE")); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), ".last_digest")
	}
	return ".last_digest"
}

// readDigestSent возвращает время последней отправки; ok=false, если файла нет
// (первый запуск) или он повреждён.
func readDigestSent(loc *time.Location) (time.Time, bool) {
	raw, err := os.ReadFile(stateFilePath())
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
	if err != nil {
		return time.Time{}, false
	}
	return t.In(loc), true
}

// recordDigestSent фиксирует момент успешной отправки. Ошибку только логировать
// не нужно — вызывающий сам решает; здесь просто пишем атомарно через temp+rename.
func recordDigestSent(t time.Time) error {
	path := stateFilePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(t.Format(time.RFC3339)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
