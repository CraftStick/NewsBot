package telegram

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearPending(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = os.MkdirAll(pendingDir, 0o700)
	for _, n := range []string{"a.json", "a.jpg", "a.at"} {
		_ = os.WriteFile(filepath.Join(pendingDir, n), []byte("x"), 0o600)
	}
	clearPending()
	if f, _ := filepath.Glob(filepath.Join(pendingDir, "*")); len(f) != 0 {
		t.Fatalf("остались файлы: %v", f)
	}
}
