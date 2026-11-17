package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

// SendMini шлёт мини-пост в личку: фото с подписью (photo == nil — просто текст),
// с кнопкой «Опубликовать», если она настроена. Предпросмотр ссылки отключён.
func SendMini(cfg config.Config, html string, photo []byte) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	tc.setPublishButton(html, photo)
	var kb *tgbotapi.InlineKeyboardMarkup
	if tc.publishKB != nil {
		k := tgbotapi.NewInlineKeyboardMarkup(*tc.publishKB)
		kb = &k
	}
	if len(photo) > 0 {
		msg := tgbotapi.NewPhoto(tc.chatID, tgbotapi.FileBytes{Name: "news.jpg", Bytes: photo})
		msg.ParseMode, msg.Caption = tgbotapi.ModeHTML, html
		if kb != nil {
			msg.ReplyMarkup = *kb
		}
		_, err = tc.bot.Send(msg)
		return err
	}
	msg := tgbotapi.NewMessage(tc.chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	_, err = tc.bot.Send(msg)
	return err
}
