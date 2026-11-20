package telegram

import (
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	screenMu    sync.Mutex
	screen      = map[int64]int{}   // чат → сообщение-«экран» (статус, отсеянные, меню): правим на месте
	pendMsgs    = map[int64][]int{} // чат → сообщения со ждущими постами от прошлого «Ожидают»
	controlCmds = map[string]bool{
		"/start": true, "/help": true, "меню": true, "/status": true, "/статус": true, "статус": true,
		"/skipped": true, "/отсеянные": true, "отсеянные": true, "/pending": true, "/ожидают": true, "ожидают": true,
	}
)

// sendPending присылает каждую ждущую заготовку целиком с кнопками «Опубликовать/Пропустить».
func (tc *telegramController) sendPending(chatID int64) {
	ps := pendingList()
	if len(ps) == 0 {
		tc.sendMenu(chatID, "Ничего не ждёт решения.")
		return
	}
	screenMu.Lock()
	for _, id := range pendMsgs[chatID] {
		_, _ = tc.bot.Request(tgbotapi.NewDeleteMessage(chatID, id))
	}
	pendMsgs[chatID] = nil
	screenMu.Unlock()
	for _, p := range ps {
		msg := tgbotapi.NewMessage(chatID, p.html)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📣 Опубликовать", cbPublish+p.key)),
			pendingRow(p.key))
		if sent, err := tc.bot.Send(msg); err == nil {
			screenMu.Lock()
			pendMsgs[chatID] = append(pendMsgs[chatID], sent.MessageID)
			screenMu.Unlock()
		}
	}
}

// sendMenu показывает «экран»: правит прошлое сообщение вместо нового, чтобы чат не пух.
func (tc *telegramController) sendMenu(chatID int64, html string) {
	screenMu.Lock()
	defer screenMu.Unlock()
	if id, ok := screen[chatID]; ok {
		e := tgbotapi.NewEditMessageText(chatID, id, html)
		e.ParseMode = tgbotapi.ModeHTML
		e.DisableWebPagePreview = true
		if _, err := tc.bot.Request(e); err == nil || strings.Contains(err.Error(), "not modified") {
			return
		}
	}
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
	if sent, err := tc.bot.Send(msg); err == nil {
		screen[chatID] = sent.MessageID
	}
}
