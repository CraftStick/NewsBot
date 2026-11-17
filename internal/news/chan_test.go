package news

import "testing"

func TestTidyPost(t *testing.T) {
	in := `<b>🤗</b><b> Выдыхайте</b><br/><br/>Дуров объяснил.<br/><br/>Почему не стоит паниковать:<br/><br/>↖️ <a href="https://kod.ru/x">https://kod.ru/x</a><br/><i><br/>* внесён в перечень</i>`
	want := "🤗 Выдыхайте\n\nДуров объяснил."
	if got := tidyPost(in); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestLeadEmoji(t *testing.T) {
	m := reLeadEmoji.FindStringSubmatch(`<b><tg-emoji emoji-id="535"><i class="emoji" style="x"><b>🤗</b></i></tg-emoji></b><b> Выдыхайте</b>`)
	if m == nil || m[1] != "535" || m[2] != "🤗" {
		t.Fatalf("got %q", m)
	}
}
