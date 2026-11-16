// Package telegram отправляет превью в личку и обслуживает кнопку «Другой
// дайджест» и команду /digest.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

const cbRegenerateDigest = "regenerate_digest"

type telegramController struct {
	cfg        config.Config
	bot        *tgbotapi.BotAPI
	chatID     int64
	mu         sync.Mutex
	regenerate func() error                     // пересборка дайджеста по кнопке и /digest
	publishKB  *[]tgbotapi.InlineKeyboardButton // кнопка «Опубликовать» для превью
}

func newTelegramController(cfg config.Config) (*telegramController, error) {
	chatID, err := config.ParsePreviewChatID(cfg.TelegramPreviewChatID)
	if err != nil {
		return nil, err
	}
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN неверный — проверьте .env")
		}
		return nil, fmt.Errorf("telegram: %w", err)
	}
	return &telegramController{cfg: cfg, bot: bot, chatID: chatID}, nil
}

func regenerateKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Другой дайджест", cbRegenerateDigest),
		),
	)
}

// previewKeyboard — кнопки под превью: «Опубликовать» (если есть) и «Другой дайджест».
func (tc *telegramController) previewKeyboard() tgbotapi.InlineKeyboardMarkup {
	kb := regenerateKeyboard()
	if tc.publishKB != nil {
		kb.InlineKeyboard = append([][]tgbotapi.InlineKeyboardButton{*tc.publishKB}, kb.InlineKeyboard...)
	}
	return kb
}

func (tc *telegramController) allowed(chatID int64) bool {
	return chatID == tc.chatID
}

func (tc *telegramController) sendHTML(chatID int64, html string, withRegenerateBtn bool) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if withRegenerateBtn {
		msg.ReplyMarkup = regenerateKeyboard()
	} else {
		msg.ReplyMarkup = tc.previewKeyboard()
	}
	sent, err := tc.bot.Send(msg)
	if err != nil {
		return err
	}
	log.Printf("message_id=%d", sent.MessageID)
	return nil
}

// sendPhotoCaption шлёт фото (из байтов) с HTML-подписью. Пустая подпись — просто фото.
func (tc *telegramController) sendPhotoCaption(chatID int64, photo []byte, htmlCaption string) error {
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "news.jpg", Bytes: photo})
	if htmlCaption != "" {
		msg.ParseMode = tgbotapi.ModeHTML
		msg.Caption = htmlCaption
	}
	if htmlCaption != "" {
		msg.ReplyMarkup = tc.previewKeyboard()
	}
	sent, err := tc.bot.Send(msg)
	if err != nil {
		return err
	}
	log.Printf("photo message_id=%d", sent.MessageID)
	return nil
}

func (tc *telegramController) sendPlain(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := tc.bot.Send(msg); err != nil {
		log.Printf("telegram: %v", err)
	}
}

// sendErrorNotice — простое (без HTML, чтобы текст ошибки ничего не сломал)
// сообщение с кнопкой пересборки: и уведомление, и напоминание собрать вручную.
func (tc *telegramController) sendErrorNotice(text string) {
	msg := tgbotapi.NewMessage(tc.chatID, text)
	msg.ReplyMarkup = regenerateKeyboard()
	if _, err := tc.bot.Send(msg); err != nil {
		log.Printf("telegram: уведомление об ошибке не отправлено: %v", err)
	}
}

// NotifyFailure шлёт владельцу превью сообщение, если дайджест по
// расписанию не собрался — иначе о провале узнать неоткуда (только логи).
func NotifyFailure(cfg config.Config, cause error) {
	tc, err := newTelegramController(cfg)
	if err != nil {
		log.Printf("не удалось создать бота для уведомления об ошибке: %v", err)
		return
	}
	tc.sendErrorNotice("❌ Пятничный дайджест не собрался автоматически.\n\n" +
		"Причина: " + cause.Error() + "\n\n" +
		"Нажмите кнопку ниже или отправьте /digest, чтобы собрать вручную.")
}

// NotifyWarning шлёт короткое предупреждение, не прерывая работу.
func NotifyWarning(cfg config.Config, text string) {
	tc, err := newTelegramController(cfg)
	if err != nil {
		log.Printf("предупреждение не отправлено: %v", err)
		return
	}
	if err := tc.sendHTML(tc.chatID, text, true); err != nil {
		log.Printf("предупреждение не отправлено: %v", err)
	}
}

func (tc *telegramController) startRegenerate(chatID int64, msgID int) {
	tc.startJob(chatID, msgID, "дайджест", tc.regenerate)
}

func (tc *telegramController) startJob(chatID int64, msgID int, what string, run func() error) {
	if !tc.allowed(chatID) {
		tc.sendPlain(chatID, "Этот бот доступен только владельцу превью.")
		return
	}
	if !tc.mu.TryLock() {
		tc.sendPlain(chatID, "⏳ Уже собираю, подождите…")
		return
	}
	go func() {
		defer tc.mu.Unlock()
		// Одно сообщение-статус: потом правим его в ошибку или убираем, когда пришло превью.
		text := "⏳ Собираю " + what + " (RSS + Gemini)…"
		var status tgbotapi.Message
		var err error
		if msgID != 0 { // нажата кнопка: правим то самое сообщение; у фото-превью это не выйдет
			status, err = tc.bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, text))
		}
		if msgID == 0 || err != nil {
			status, err = tc.bot.Send(tgbotapi.NewMessage(chatID, text))
		}
		if err != nil {
			log.Printf("telegram: %v", err)
		}
		if err := run(); err != nil {
			log.Printf("%s: %v", what, err)
			text := "❌ Ошибка: " + err.Error()
			if status.MessageID == 0 {
				tc.sendPlain(chatID, text)
				return
			}
			edit := tgbotapi.NewEditMessageText(chatID, status.MessageID, text)
			kb := regenerateKeyboard()
			edit.ReplyMarkup = &kb
			if _, err := tc.bot.Send(edit); err != nil {
				log.Printf("telegram: %v", err)
			}
			return
		}
		if status.MessageID != 0 {
			_, _ = tc.bot.Request(tgbotapi.NewDeleteMessage(chatID, status.MessageID))
		}
	}()
}

// handlePublish публикует и отвечает всплывающим окном, без новых сообщений в чате.
func (tc *telegramController) handlePublish(q *tgbotapi.CallbackQuery) {
	cb := tgbotapi.NewCallback(q.ID, "✅ Опубликовано в канале")
	cb.ShowAlert = true
	if err := publishPending(tc.cfg, strings.TrimPrefix(q.Data, cbPublish)); err != nil {
		log.Printf("публикация: %v", err)
		cb.Text = "❌ Не опубликовано: " + err.Error()
		if r := []rune(cb.Text); len(r) > 200 {
			cb.Text = string(r[:199]) + "…"
		}
	} else {
		log.Print("пост опубликован юзерботом")
		// публикующую кнопку убираем, чтобы не нажали дважды; «Другой дайджест» оставляем
		kb := regenerateKeyboard()
		_, _ = tc.bot.Request(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, kb))
	}
	if _, err := tc.bot.Request(cb); err != nil {
		log.Printf("telegram: ответ на кнопку: %v", err)
	}
}

func (tc *telegramController) handleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		q := update.CallbackQuery
		if strings.HasPrefix(q.Data, cbPublish) && q.Message != nil && tc.allowed(q.Message.Chat.ID) {
			tc.handlePublish(q)
			return
		}
		if q.Data == cbRegenerateDigest && q.Message != nil && tc.allowed(q.Message.Chat.ID) {
			_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "Запускаю новый дайджест…"))
			tc.startRegenerate(q.Message.Chat.ID, q.Message.MessageID)
		}
		return
	}

	if update.Message == nil || update.Message.Text == "" {
		return
	}
	chatID := update.Message.Chat.ID
	if !tc.allowed(chatID) {
		return
	}

	switch strings.TrimSpace(strings.ToLower(update.Message.Text)) {
	case "/start", "/help":
		const welcome = "Привет! Я собираю пятничный дайджест.\n\n" +
			"<b>/digest</b> — собрать новый дайджест\n" +
			"Или нажмите кнопку ниже"
		_ = tc.sendHTML(chatID, welcome, true)
	case "/digest", "/new", "/новости", "/дайджест":
		tc.startRegenerate(chatID, 0)
	}
}

// errPollingTaken — опрос уже ведёт другой процесс (обычно systemd-сервис).
// Telegram отдаёт getUpdates только одному клиенту, второй получает Conflict.
var errPollingTaken = errors.New("опрос обновлений уже ведёт другой экземпляр бота")

// pollingAvailable проверяет, свободен ли getUpdates, ОДНИМ коротким запросом.
// Без этого второй процесс уходит в бесконечную драку с первым и раз в три
// секунды сыплет в лог «Conflict: terminated by other getUpdates request».
func (tc *telegramController) pollingAvailable() error {
	u := tgbotapi.NewUpdate(-1)
	u.Timeout = 0
	u.Limit = 1
	if _, err := tc.bot.GetUpdates(u); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "conflict") {
			return errPollingTaken
		}
		return err
	}
	return nil
}

func Run(ctx context.Context, cfg config.Config, regenerate func() error) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	tc.regenerate = regenerate
	log.Printf("Telegram: @%s (кнопка, /digest)", tc.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tc.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			tc.handleUpdate(update)
		}
	}
}

func RunUntilSignal(cfg config.Config, regenerate func() error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		cancel()
	}()

	// Разовый прогон уступает опрос долгоживущему сервису: иначе два процесса
	// бесконечно выбивают друг друга из getUpdates и засыпают лог Conflict-ами.
	tc, err := newTelegramController(cfg)
	if err != nil {
		log.Printf("telegram bot: %v", err)
		return
	}
	if err := tc.pollingAvailable(); errors.Is(err, errPollingTaken) {
		log.Println("Кнопку «Другой дайджест» обслуживает запущенный сервис — второй опрос не нужен. " +
			"Дайджест отправлен, выхожу.")
		return
	}

	if err := Run(ctx, cfg, regenerate); err != nil {
		log.Printf("telegram bot: %v", err)
	}
}
