package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

// SendMini шлёт мини-пост в личку; с кнопкой «Опубликовать», если она настроена.
func SendMini(cfg config.Config, html string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	tc.setPublishButton(html, nil)
	msg := tgbotapi.NewMessage(tc.chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	if tc.publishKB != nil {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(*tc.publishKB)
	}
	_, err = tc.bot.Send(msg)
	return err
}
