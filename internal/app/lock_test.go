package app

import (
	"testing"

	"treesheild-newsbot/internal/config"
)

func TestRunDigestRejectsConcurrent(t *testing.T) {
	digestMu.Lock()
	defer digestMu.Unlock()
	if err := RunDigest(config.Config{}); err == nil {
		t.Fatal("ожидалась ошибка: дайджест уже собирается")
	}
}
