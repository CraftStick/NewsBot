package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestAssembleDigestForCaption(t *testing.T) {
	t.Parallel()

	// Длинный дайджест: 6 пунктов по 2 длинных предложения — заведомо больше лимита.
	long := strings.Repeat("Первое длинное предложение про блокировки VPN и Роскомнадзор в России. "+
		"Второе не менее длинное предложение с деталями и подробностями события недели. ", 1)
	var b strings.Builder
	for i := 1; i <= requiredNewsItems; i++ {
		fmt.Fprintf(&b, "<b>%d. <a href=\"https://example.com/%d\">Заголовок новости номер %d про рунет</a></b>\n%s\n\n", i, i, i, long)
	}
	body := strings.TrimSpace(b.String())

	// Полный дайджест действительно превышает лимит подписи.
	if got := captionVisibleLen(assembleDigest(body)); got <= telegramMaxCaption {
		t.Fatalf("ожидали, что полный дайджест длиннее лимита, а он %d", got)
	}

	caption, ok := assembleDigestForCaption(body)
	if !ok {
		t.Fatal("не удалось ужать дайджест под подпись")
	}
	if n := captionVisibleLen(caption); n > telegramMaxCaption {
		t.Fatalf("подпись после сжатия всё ещё длинная: %d > %d", n, telegramMaxCaption)
	}
	// Все 6 пунктов и ссылки должны сохраниться.
	if n := countNewsItems(caption); n < requiredNewsItems {
		t.Fatalf("после сжатия потеряны пункты: %d из %d", n, requiredNewsItems)
	}
	if strings.Count(caption, "href=") < requiredNewsItems {
		t.Fatalf("после сжатия потеряны ссылки")
	}

	// Короткий дайджест не должен меняться (влезает целиком).
	short := "<b>1. <a href=\"https://e.com/1\">Кратко</a></b>\nОдно предложение."
	if _, ok := assembleDigestForCaption(short); !ok {
		t.Fatal("короткий дайджест должен влезать в подпись")
	}
}
