package telegram

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

// Отложенная публикация: время хранится в pending/<key>.at (unix-секунды), поэтому
// переживает перезапуск; раз в полминуты runScheduler публикует созданные, но не ушедшие посты.
const (
	cbLater = "later:" // later:<key> — показать варианты времени
	cbAt    = "at:"    // at:<key>:<1h|3h|am> — запланировать
	// morningHour — «утром»: ближайшие 09:00 по часовому поясу бота.
	morningHour = 9
)

// laterTime считает момент публикации для варианта opt; false — неизвестный вариант.
func laterTime(opt string, now time.Time, loc *time.Location) (time.Time, bool) {
	switch opt {
	case "1h":
		return now.Add(time.Hour), true
	case "3h":
		return now.Add(3 * time.Hour), true
	case "am":
		n := now.In(loc)
		t := time.Date(n.Year(), n.Month(), n.Day(), morningHour, 0, 0, 0, loc)
		if !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}
	return time.Time{}, false
}

func atPath(key string) string { return filepath.Join(pendingDir, key+".at") }

func laterKeyboard(key string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("через 1 ч", cbAt+key+":1h"),
			tgbotapi.NewInlineKeyboardButtonData("через 3 ч", cbAt+key+":3h"),
			tgbotapi.NewInlineKeyboardButtonData("утром 09:00", cbAt+key+":am")),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📣 Опубликовать сейчас", cbPublish+key),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Пропустить", cbSkip+key)))
}

func (tc *telegramController) handleLater(q *tgbotapi.CallbackQuery) {
	key := strings.TrimPrefix(q.Data, cbLater)
	if !pendingKeyRE.MatchString(key) {
		return
	}
	_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "Когда опубликовать?"))
	_, _ = tc.bot.Request(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, laterKeyboard(key)))
}

func (tc *telegramController) handleAt(q *tgbotapi.CallbackQuery) {
	parts := strings.Split(strings.TrimPrefix(q.Data, cbAt), ":")
	if len(parts) != 2 || !pendingKeyRE.MatchString(parts[0]) {
		return
	}
	key := parts[0]
	when, ok := laterTime(parts[1], time.Now(), tc.cfg.Timezone)
	if _, err := os.Stat(filepath.Join(pendingDir, key+".json")); !ok || err != nil {
		_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "Пост уже опубликован или устарел"))
		return
	}
	if err := os.WriteFile(atPath(key), []byte(strconv.FormatInt(when.Unix(), 10)), 0o600); err != nil {
		_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "❌ Не удалось запланировать"))
		return
	}
	text := "⏰ Опубликую " + when.In(tc.cfg.Timezone).Format("02.01 в 15:04")
	_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, text))
	// «Опубликовать сейчас» и «Пропустить» (отмена) остаются
	kb := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(text, "noop"),
		tgbotapi.NewInlineKeyboardButtonData("🗑 Отменить", cbSkip+key)))
	_, _ = tc.bot.Request(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, kb))
}

// runScheduler публикует посты, чьё время пришло. Ошибка не повторяется: файл времени
// удаляется, а в чат приходит сообщение — иначе падающий пост уйдёт в бесконечный цикл.
func (tc *telegramController) runScheduler(cfg config.Config, stop <-chan struct{}) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			tc.publishDue(cfg, time.Now())
		}
	}
}

func (tc *telegramController) publishDue(cfg config.Config, now time.Time) {
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*.at"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		unix, perr := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if err != nil || perr != nil || time.Unix(unix, 0).After(now) {
			continue
		}
		key := strings.TrimSuffix(filepath.Base(f), ".at")
		if err := publishPending(cfg, key); err != nil {
			_ = os.Remove(f)
			log.Printf("отложенная публикация: %v", err)
			_ = tc.sendHTML(tc.chatID, fmt.Sprintf("❌ Отложенный пост не опубликован: %v", err), true)
			continue
		}
		log.Print("отложенный пост опубликован юзерботом")
		_ = tc.sendHTML(tc.chatID, "✅ Отложенный пост опубликован", true)
	}
}
