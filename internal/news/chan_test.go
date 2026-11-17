package news

import "testing"

func TestTidyPost(t *testing.T) {
	in := `<b>🤗</b><b> Выдыхайте</b><br/><br/>Дуров объяснил.<br/><br/>Почему не стоит паниковать:<br/><br/>↖️ <a href="https://kod.ru/x">https://kod.ru/x</a><br/><i><br/>* внесён в перечень</i>`
	want := "🤗 Выдыхайте\n\nДуров объяснил."
	if got := tidyPost(in); got != want {
		t.Fatalf("got %q", got)
	}
}
