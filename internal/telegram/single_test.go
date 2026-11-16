package telegram

import "testing"

func TestFormatSinglePost(t *testing.T) {
	in := "<b>1. <a href=\"https://x/1\">Beget получил DDoS после взлома</a></b>\nТекст один. Текст два."
	want := "🛡️ <b><a href=\"https://x/1\">Beget получил DDoS после взлома</a></b>\n\nТекст один. Текст два."
	if got := formatSinglePost(in); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestPickEmojiID(t *testing.T) {
	if _, id := PickEmoji("Новый VPN под блокировкой"); id != "5767151002666929821" { // блокировка раньше VPN
		t.Fatalf("id=%s", id)
	}
	if _, id := PickEmoji("Вышел смартфон"); id != defaultEmojiID {
		t.Fatalf("id=%s", id)
	}
}

func TestPickEmojiNewTopics(t *testing.T) {
	cases := map[string]string{
		"Утечка данных клиентов банка":    "5278602437001767574",
		"МТС расширила сеть":              "5803403369913520877",
		"Яндекс запустил сервис":          "5291742567321328330",
		"Google открыла офис в США":       "5222258908529207849",
		"Роскомнадзор разблокировал сайт": "5278411813468269386",
		"OpenAI выпустила модель":         "5785346296608988394",
	}
	for in, want := range cases {
		if _, id := PickEmoji(in); id != want {
			t.Errorf("%q: id=%s, ждали %s", in, id, want)
		}
	}
}
