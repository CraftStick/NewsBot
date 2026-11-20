package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// sendPending присылает каждую ждущую заготовку целиком с кнопками «Опубликовать/Пропустить».
func (tc *telegramController) sendPending(chatID int64) {
	ps := pendingList()
	if len(ps) == 0 {
		tc.sendMenu(chatID, "Ничего не ждёт решения.")
		return
	}
	for _, p := range ps {
		msg := tgbotapi.NewMessage(chatID, p.html)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📣 Опубликовать", cbPublish+p.key),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Пропустить", cbSkip+p.key)))
		_, _ = tc.bot.Send(msg)
	}
}

// sendMenu шлёт сообщение и ставит постоянную клавиатуру; тексты кнопок
// разбирает switch в handleUpdate (в нижнем регистре).
func (tc *telegramController) sendMenu(chatID int64, html string) {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Статус"), tgbotapi.NewKeyboardButton("Отсеянные")),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Дайджест"), tgbotapi.NewKeyboardButton("Пост")),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Ожидают")),
	)
	kb.ResizeKeyboard = true
	msg.ReplyMarkup = kb
	_, _ = tc.bot.Send(msg)
}
