package telegram

import "testing"

func TestDislike(t *testing.T) {
	t.Chdir(t.TempDir())
	key, _ := savePending(`<b>Тема</b> <a href="https://x.ru/1">ссылка</a>`, "", "")
	noteDisliked(key)
	skipPending(key)
	if dislikedCount() != 1 || len(pendingList()) != 0 {
		t.Fatalf("записей %d, в очереди %d", dislikedCount(), len(pendingList()))
	}
}
