package news

import "testing"

func TestIsRoundup(t *testing.T) {
	if !isRoundup("🐈 Компании на kod.ru") || isRoundup("VK подала иск к Apple") {
		t.Fatal("isRoundup")
	}
}
