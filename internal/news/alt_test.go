package news

import (
	"context"
	"testing"
)

func TestChannelPostFallsBackToSiteImage(t *testing.T) {
	in := []Article{
		{Source: "Код Дурова", Link: "https://kod.ru/a/", Image: "https://kod.ru/a.jpg"},
		{Source: "tg", Link: "https://t.me/kod_ru/1", Ref: refKey("https://kod.ru/a")},
	}
	out := dropRefDupes(in)
	if len(out) != 1 || out[0].Alt == nil || out[0].Alt.Image != "https://kod.ru/a.jpg" {
		t.Fatalf("Alt не проставлен: %+v", out)
	}
	post := out[0]
	post.Link = "" // без сети: своего фото нет
	if got := ImageFor(context.Background(), post); got != "https://kod.ru/a.jpg" {
		t.Fatalf("ждали картинку сайта, получили %q", got)
	}
	post.Image = "https://t.me/own.jpg"
	if got := ImageFor(context.Background(), post); got != post.Image {
		t.Fatalf("фото поста должно быть приоритетнее, получили %q", got)
	}
}
