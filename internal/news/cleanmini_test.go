package news

import "testing"

func TestCleanMiniTextTruncatedTitle(t *testing.T) {
	title, sum := cleanMiniText("⛔️ «Боты принимают только Stars (Grams) за оплату из-за правил Apple…",
		"⛔️ «Боты принимают только Stars (Grams*) за оплату из-за правил Apple и Google. Дальше текст.")
	if title != "«Боты принимают только Stars (Grams*) за оплату из-за правил Apple и Google." || sum != "Дальше текст." {
		t.Fatalf("title=%q sum=%q", title, sum)
	}
}
