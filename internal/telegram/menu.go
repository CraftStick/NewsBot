package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// sendMenu шлёт сообщение и ставит постоянную клавиатуру; тексты кнопок
// разбирает switch в handleUpdate (в нижнем регистре).
func (tc *telegramController) sendMenu(chatID int64, html string) {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Статус"), tgbotapi.NewKeyboardButton("Отсеянные")),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Дайджест"), tgbotapi.NewKeyboardButton("Пост")),
	)
	kb.ResizeKeyboard = true
	msg.ReplyMarkup = kb
	_, _ = tc.bot.Send(msg)
}
