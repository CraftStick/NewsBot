package news

import "testing"

func TestCleanMiniText(t *testing.T) {
	tt, ss := cleanMiniText("🐶 Дуров: «700 000 кошельков».", "🐶 Дуров: «700 000 кошельков».")
	if tt != "Дуров: «700 000 кошельков»." || ss != "" {
		t.Fatalf("%q %q", tt, ss)
	}
	if _, ss = cleanMiniText("Заголовок", "Другой текст"); ss != "Другой текст" {
		t.Fatal(ss)
	}
}
