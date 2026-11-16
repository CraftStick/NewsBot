package digest

import (
	"strings"
	"testing"

	"treesheild-newsbot/internal/news"
)

func TestTitleMatchScore(t *testing.T) {
	t.Parallel()
	a := normalizeTitle("Роскомнадзор убрал планы блокировки VPN")
	b := normalizeTitle("Роскомнадзор вырезал из документов раздел с планами блокировки VPN")
	if titleMatchScore(a, b) < 12 {
		t.Fatalf("expected match, score=%d", titleMatchScore(a, b))
	}
	if titleMatchScore("abc", "xyz") != 0 {
		t.Fatal("unrelated titles should not match")
	}
}

func TestEnsureNewsLinks(t *testing.T) {
	t.Parallel()
	articles := []news.Article{
		{Title: "РКН и VPN", Link: "https://example.com/rkn", Source: "Test", RUPriority: 2},
		{Title: "Foreign leak", Link: "https://example.com/en", Source: "EN", RUPriority: 0},
	}
	body := `<b>1. РКН и VPN</b>
Первое предложение про блокировки и VPN в России сегодня. Второе предложение с подробностями для читателя.

<b>2. Foreign leak</b>
Foreign leak story sentence here today. Second sentence adds international context now.`
	out := EnsureNewsLinks(body, articles)
	blocks := splitNewsBlocks(out)
	if len(blocks) != 2 {
		t.Fatalf("blocks: got %d", len(blocks))
	}
	for i, block := range blocks {
		if extractNewsURL(block) == "" {
			t.Fatalf("block %d without URL", i+1)
		}
	}
}

func TestEnsureNewsLinksNoRandomFallback(t *testing.T) {
	arts := []news.Article{
		{Title: "ГК Softline: сотрудники не должны бояться кибершантажа", Link: "https://kod.ru/softline"},
		{Title: "Ответственность за сбой «Дзена» взяла на себя кибергруппировка iEM Group", Link: "https://am.ru/zen"},
	}
	post := "<b>1. Атака на «Дзен»</b>\nКибергруппировка iEM Group взяла ответственность за сбой платформы «Дзен».\n\n<b>2. Совсем другая тема</b>\nТекст про что-то иное без совпадений."
	got := EnsureNewsLinks(post, arts)
	if !strings.Contains(got, `href="https://am.ru/zen"`) {
		t.Fatalf("Дзен не сопоставлен: %s", got)
	}
	if strings.Contains(got, "kod.ru/softline") {
		t.Fatalf("подставлена случайная статья: %s", got)
	}
}

func TestTitleMatchScoreNeedsCoverage(t *testing.T) {
	q := normalizeTitle("Проверка работы VPN-сервисов")
	a := normalizeTitle("Apple закроет форум поддержки: он работал с 1998 года, сервисов больше не будет")
	if s := titleMatchScore(q, a); s >= minLinkScore {
		t.Fatalf("случайное совпадение прошло: %d", s)
	}
}
