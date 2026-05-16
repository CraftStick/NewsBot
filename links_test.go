package main

import "testing"

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
	articles := []Article{
		{Title: "РКН и VPN", Link: "https://example.com/rkn", Source: "Test", RUPriority: 2},
		{Title: "Foreign leak", Link: "https://example.com/en", Source: "EN", RUPriority: 0},
	}
	body := `<b>1. РКН и VPN</b>
Первое предложение про блокировки и VPN в России сегодня. Второе предложение с подробностями для читателя.

<b>2. Зарубеж</b>
First foreign privacy story sentence here today. Second sentence adds international context now.`
	out := ensureNewsLinks(body, articles)
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
