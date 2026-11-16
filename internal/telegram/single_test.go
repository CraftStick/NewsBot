package telegram

import "testing"

func TestFormatSinglePost(t *testing.T) {
	in := "<b>1. <a href=\"https://x/1\">Beget получил DDoS после взлома</a></b>\nТекст один. Текст два."
	want := "🛡 <b><a href=\"https://x/1\">Beget получил DDoS после взлома</a></b>\n\nТекст один. Текст два."
	if got := formatSinglePost(in); got != want {
		t.Fatalf("got %q", got)
	}
}
