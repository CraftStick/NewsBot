package main

import (
	"fmt"
	"strings"
	"testing"
)

func makeBody(items [][2]string) string {
	var b strings.Builder
	for i, it := range items {
		fmt.Fprintf(&b, "<b>%d. <a href=\"https://example.com/%d\">%s</a></b>\n%s\n\n", i+1, i+1, it[0], it[1])
	}
	return strings.TrimSpace(b.String())
}

// Сжатие подписи режет только по границам предложений: тело каждого пункта в
// результате — либо исходное целиком, либо его первое предложение. Никаких «…».
func TestCaptionKeepsCompleteSentences(t *testing.T) {
	t.Parallel()
	items := [][2]string{
		{"Масштабный сбой в Рунете из-за Ростелекома", "Российский интернет пережил масштабный сбой 6 августа, затронувший десятки сервисов. Причиной стали неполадки в сетях «Ростелекома»."},
		{"Госуслуги станут социальной медиаплатформой", "Минцифры планирует превратить портал в полноценную соцсеть. Отдельного мессенджера при этом не будет."},
		{"Минцифры опровергло запрет соцсетей для детей", "Ведомство опровергло планы ограничить детям доступ к соцсетям. Такое решение остаётся за родителями."},
		{"Яндекс объяснил работу Алисы до команды", "Компания прокомментировала опасения о прослушивании. Алиса ловит звук до команды по техническим причинам."},
		{"ФСБ задержала пособников мошенников в Сити", "В Москва-Сити ликвидировали девять точек нелегального обмена. Задержаны их организаторы."},
		{"Apple временно удалила Telegram из App Store", "Apple на несколько часов убрала мессенджер из магазина. Позже приложение вернулось."},
	}
	body := makeBody(items)

	fitted, ok := fitNewsBodyToCaption(body, telegramMaxCaption)
	if !ok {
		t.Fatal("ожидали, что дайджест влезает хотя бы по одному предложению")
	}
	if n := captionVisibleLen(assembleDigest(fitted)); n > telegramMaxCaption {
		t.Fatalf("подпись длиннее лимита: %d", n)
	}
	// Каждое тело — целиком либо ровно первое предложение (без обрыва слова).
	origBlocks := splitNewsBlocks(body)
	for i, fb := range splitNewsBlocks(fitted) {
		got := newsBlockBody(fb)
		full := newsBlockBody(origBlocks[i])
		if got != full && got != firstSentence(full) {
			t.Fatalf("пункт %d обрезан не по границе предложения: %q", i+1, got)
		}
		if strings.HasSuffix(got, "…") {
			t.Fatalf("пункт %d оборван на полуслове: %q", i+1, got)
		}
	}
}

// Слишком длинный дайджест не помещается даже по одному предложению — тогда
// fit возвращает false (вызывающий шлёт фото и полный текст раздельно).
func TestCaptionFallsBackWhenTooLong(t *testing.T) {
	t.Parallel()
	long := "Одно очень длинное и подробное предложение про блокировки VPN, Роскомнадзор, Минцифры и рунет с массой деталей и уточнений на всякий случай."
	items := make([][2]string, requiredNewsItems)
	for i := range items {
		items[i] = [2]string{"Очень длинный заголовок новости номер про рунет и VPN", long + " " + long}
	}
	if _, ok := fitNewsBodyToCaption(makeBody(items), telegramMaxCaption); ok {
		t.Fatal("ожидали fallback (ok=false) для слишком длинного дайджеста")
	}
}

func TestFirstSentence(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Первое предложение. Второе предложение.": "Первое предложение.",
		"Без точки в конце":                       "Без точки в конце",
		"Вопрос? И ответ.":                        "Вопрос?",
	}
	for in, want := range cases {
		if got := firstSentence(in); got != want {
			t.Fatalf("firstSentence(%q) = %q, ожидали %q", in, got, want)
		}
	}
}
