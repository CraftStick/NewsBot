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

// topicEmoji — эмодзи по теме. ID — анимированный custom emoji для юзербота
// (бот их отправить не может, в превью идёт обычный символ).
var topicEmoji = []struct {
	emoji, id string
	words     []string
}{
	{"✅", "5278411813468269386", []string{"разблокир", "сняли блокир", "снял блокир", "оправдал"}},
	{"🚫", "5767151002666929821", []string{"блокир", "заблокир", "роскомнадзор", "ркн", "запрет", "ограничен"}},
	{"🔓", "5278602437001767574", []string{"утечк", "слив", "персональн"}},
	{"🛸", "5350513460985743273", []string{"vpn", "впн", "шифрован", "приватн"}},
	{"🛡️", "5348230282140993341", []string{"хакер", "взлом", "уязвим", "ddos", "атак", "вирус", "malware", "фишинг", "мошенн"}},
	{"🤖", "5785346296608988394", []string{"нейросет", "искусственн", "chatgpt", "openai", "gpt", "llm"}},
	{"📱", "5348482508390419372", []string{"telegram", "телеграм", "whatsapp", "мессенджер"}},
	{"☕️", "5348414471813483004", []string{"суд", "штраф", "закон", "госдум"}},
	{"🔍", "5803403369913520877", []string{"оператор", "мтс", "билайн", "мегафон", "теле2", "мобильн", "связ"}},
	{"🔎", "5291742567321328330", []string{"яндекс", "vk", "вк ", "сбер", "госуслуг", "ozon", "wildberries"}},
	{"🌐", "5222258908529207849", []string{"сша", "китай", "евросоюз", "европ", "apple", "google", "microsoft", "meta", "amazon", "nvidia", "tesla"}},
}

// defaultEmoji — когда тема не распознана.
const (
	defaultEmoji   = "📰"
	defaultEmojiID = "5951891646445523477"
)

// PickEmoji возвращает символ и id анимированного эмодзи по теме текста.
func PickEmoji(text string) (emoji, id string) {
	text = strings.ToLower(text)
	for _, t := range topicEmoji {
		for _, w := range t.words {
			if strings.Contains(text, w) {
				return t.emoji, t.id
			}
		}
	}
	return defaultEmoji, defaultEmojiID
}

// formatSinglePost: «эмодзи заголовок», пустая строка, текст.
func formatSinglePost(post string) string {
	out := headingNumRE.ReplaceAllString(strings.TrimSpace(post), "<b>")
	out = strings.Replace(out, "</b>\n", "</b>\n\n", 1)
	e, _ := PickEmoji(out)
	return e + " " + out
}
