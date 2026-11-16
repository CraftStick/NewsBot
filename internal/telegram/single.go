package telegram

import (
	"regexp"
	"strings"

	"treesheild-newsbot/internal/config"
)

var headingNumRE = regexp.MustCompile(`^<b>\s*\d{1,2}\.\s*`)

// PublishSinglePreview шлёт одиночный пост ТОЛЬКО в личку на проверку: в канал
// бот сам ничего не отправляет, пост копируется вручную.
func PublishSinglePreview(cfg config.Config, post string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	if err := tc.sendHTML(tc.chatID, "<i>Пост на сегодня (превью).</i> Подходит — скопируйте <b>следующее</b> сообщение в канал; нет — просто пропустите.", false); err != nil {
		return err
	}
	return tc.sendHTML(tc.chatID, formatSinglePost(post), false)
}

// topicEmoji — эмодзи по теме; позже заменятся анимированными (юзербот).
var topicEmoji = []struct {
	emoji string
	words []string
}{
	{"🚫", []string{"блокир", "заблокир", "роскомнадзор", "ркн", "запрет", "ограничен"}},
	{"🔐", []string{"vpn", "впн", "шифрован", "приватн", "утечк", "персональн"}},
	{"🛡", []string{"хакер", "взлом", "уязвим", "ddos", "атак", "вирус", "malware", "фишинг", "мошенн"}},
	{"📱", []string{"telegram", "телеграм", "whatsapp", "мессенджер"}},
	{"⚖️", []string{"суд", "штраф", "закон", "госдум"}},
}

func pickEmoji(text string) string {
	text = strings.ToLower(text)
	for _, t := range topicEmoji {
		for _, w := range t.words {
			if strings.Contains(text, w) {
				return t.emoji
			}
		}
	}
	return "📰"
}

// formatSinglePost: «эмодзи заголовок», пустая строка, текст.
func formatSinglePost(post string) string {
	out := headingNumRE.ReplaceAllString(strings.TrimSpace(post), "<b>")
	out = strings.Replace(out, "</b>\n", "</b>\n\n", 1)
	return pickEmoji(out) + " " + out
}
