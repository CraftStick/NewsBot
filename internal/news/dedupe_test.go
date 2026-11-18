package news

import "testing"

func TestDropRefDupes(t *testing.T) {
	in := []Article{
		{Link: "https://kod.ru/a/?utm=1"},
		{Link: "https://kod.ru/b"},
		{Link: "https://t.me/kod_ru/1", Ref: refKey("https://kod.ru/a?x=2")},
	}
	got := dropRefDupes(in)
	if len(got) != 2 || got[0].Link != "https://kod.ru/b" || got[1].Ref == "" {
		t.Fatalf("got %+v", got)
	}
}
