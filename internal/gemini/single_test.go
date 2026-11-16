package gemini

import (
	"testing"

	"treesheild-newsbot/internal/digest"
)

func TestNormalizeSingleNewsBlockFormats(t *testing.T) {
	text := "Первое предложение достаточно длинное для проверки. Второе тоже длинное предложение."
	cases := map[string]string{
		"markdown без номера": "**Роскомнадзор установил срок**\n" + text,
		"markdown с номером":  "**6. Кино по расписанию**\n" + text,
		"номер без разметки":  "6. Кино по расписанию\n" + text,
		"штатный формат":      "<b>1. Роскомнадзор установил срок</b>\n" + text,
		"заголовок без всего": "Роскомнадзор установил срок\n" + text,
	}
	for name, raw := range cases {
		body := digest.SanitizeNewsBody(normalizeSingleNewsBlock(raw, 1))
		if err := digest.ValidateSingleNewsBlock(body); err != nil {
			t.Errorf("%s: %v\n%q", name, err, body)
		}
	}
}
