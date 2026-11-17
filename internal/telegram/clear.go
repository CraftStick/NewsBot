package telegram

import (
	"log"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	cbClearAsk = "clr_ask"
	cbClearYes = "clr_yes"
	cbClearNo  = "clr_no"
)

// ClearDB — сброс памяти мини-постов; задаётся из main (пакет app сам импортирует telegram).
var ClearDB func() error

// clearPending удаляет все заготовки превью, фото и отложенные времена.
func clearPending() {
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*"))
	for _, f := range files {
		_ = os.Remove(f)
	}
}

// handleClear: первый клик просит подтверждения (чистка стирает и неопубликованные превью).
func (tc *telegramController) handleClear(q *tgbotapi.CallbackQuery) {
	chat := q.Message.Chat.ID
	switch q.Data {
	case cbClearAsk:
		m := tgbotapi.NewMessage(chat, "🧹 Очистить БД? Забуду просмотренные новости и удалю неопубликованные превью и отложенные посты. Следующий опрос просто запомнит текущую ленту.")
		m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Да, очистить", cbClearYes),
			tgbotapi.NewInlineKeyboardButtonData("Отмена", cbClearNo),
		))
		_, _ = tc.bot.Send(m)
		_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, ""))
		return
	case cbClearYes:
		text := "✅ БД очищена"
		if ClearDB == nil {
			text = "❌ очистка недоступна"
		} else if err := ClearDB(); err != nil {
			text = "❌ " + err.Error()
			log.Printf("очистка БД: %v", err)
		} else {
			clearPending()
			log.Print("БД очищена кнопкой")
		}
		_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, text))
	default:
		_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "Отменено"))
	}
	_, _ = tc.bot.Request(tgbotapi.NewDeleteMessage(chat, q.Message.MessageID))
}
