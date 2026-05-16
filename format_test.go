package main

import (
	"strings"
	"testing"
)

func TestTrimHeadline(t *testing.T) {
	t.Parallel()
	short := trimHeadline("РКН и VPN")
	if short != "РКН и VPN" {
		t.Fatalf("short: got %q", short)
	}
	long := trimHeadline(strings.Repeat("слово ", 30))
	if len([]rune(long)) > maxNewsTitleRunes+3 {
		t.Fatalf("long headline not trimmed: %d runes", len([]rune(long)))
	}
	if !strings.HasSuffix(long, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", long)
	}
	stripped := trimHeadline("Заголовок — «Интерфакс»")
	if strings.Contains(stripped, "Интерфакс") {
		t.Fatalf("source suffix not stripped: %q", stripped)
	}
}

func TestSplitNewsBlocks(t *testing.T) {
	t.Parallel()
	body := `<b>1. One</b>
text one

<b>2. Two</b>
text two`
	blocks := splitNewsBlocks(body)
	if len(blocks) != 2 {
		t.Fatalf("blocks: got %d", len(blocks))
	}
	if !strings.Contains(blocks[0], "One") {
		t.Fatal("first block missing title")
	}
}

func TestValidateNewsBody(t *testing.T) {
	t.Parallel()
	ok := `<b>1. A</b>
Первое предложение про VPN и блокировки в России. Второе предложение с деталями.

<b>2. B</b>
Первое предложение про VPN и блокировки в России. Второе предложение с деталями.

<b>3. C</b>
Первое предложение про VPN и блокировки в России. Второе предложение с деталями.

<b>4. D</b>
Первое предложение про VPN и блокировки в России. Второе предложение с деталями.

<b>5. E</b>
Первое предложение про VPN и блокировки в России. Второе предложение с деталями.

<b>6. F</b>
First sentence about privacy abroad. Second sentence with more context.`
	if err := validateNewsBody(ok); err != nil {
		t.Fatalf("valid body: %v", err)
	}
	if err := validateNewsBody(`<b>1. Only</b>
short`); err == nil {
		t.Fatal("expected error for single short item")
	}
}
