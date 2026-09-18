package main

import (
	"fmt"
	"strings"
	"testing"
)

// Все варианты формата заголовков, которые модель выдаёт вместо <b>N. …</b>.
// Каждый раньше давал «пунктов 0 из 6» и проваливал весь дайджест.
func TestNormalizeNewsHeadings(t *testing.T) {
	t.Parallel()

	item := func(format string) string {
		var b strings.Builder
		for i := 1; i <= requiredNewsItems; i++ {
			fmt.Fprintf(&b, format, i, i)
			b.WriteString("Первое предложение про событие. Второе предложение с новым фактом.\n\n")
		}
		return b.String()
	}
	cases := map[string]string{
		"как в промпте <b>1.</b> Заголовок": item("<b>%d.</b> Заголовок %d\n"),
		"markdown **1. Заголовок**":         item("**%d. Заголовок %d**\n"),
		"1. <b>Заголовок</b>":               item("%d. <b>Заголовок %d</b>\n"),
		"<strong>1. Заголовок</strong>":     item("<strong>%d. Заголовок %d</strong>\n"),
		"без разметки 1. Заголовок":         item("%d. Заголовок %d\n"),
	}
	for name, body := range cases {
		got := sanitizeNewsBody(body)
		if n := countNewsItems(got); n != requiredNewsItems {
			t.Errorf("%s: пунктов %d из %d после нормализации:\n%s", name, n, requiredNewsItems, got)
			continue
		}
		if title := extractNewsTitle(splitNewsBlocks(got)[0]); title != "Заголовок 1" {
			t.Errorf("%s: заголовок извлёкся как %q", name, title)
		}
	}
}

// Правильный формат нормализация не трогает.
func TestNormalizeKeepsCorrectFormat(t *testing.T) {
	t.Parallel()

	body := makeBody([][2]string{
		{"Раз", "Предложение один. Предложение два."}, {"Два", "Предложение один. Предложение два."},
		{"Три", "Предложение один. Предложение два."}, {"Четыре", "Предложение один. Предложение два."},
		{"Пять", "Предложение один. Предложение два."}, {"Шесть", "Предложение один. Предложение два."},
	})
	if got := normalizeNewsHeadings(body); got != body {
		t.Fatalf("корректный дайджест изменён:\n%s", got)
	}
}

// Отказ модели прозой нормализация не должна превращать в «пункты».
func TestNormalizeLeavesProseRefusal(t *testing.T) {
	t.Parallel()

	prose := "К сожалению, в ленте недостаточно новостей, которые подходят под все критерии. " +
		"Могу предложить дайджест из четырёх пунктов."
	if n := countNewsItems(sanitizeNewsBody(prose)); n != 0 {
		t.Fatalf("из отказа прозой получилось %d пунктов", n)
	}
}
