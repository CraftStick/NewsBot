package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const cbRegenerateDigest = "regenerate_digest"

type telegramController struct {
	cfg    Config
	bot    *tgbotapi.BotAPI
	chatID int64
	mu     sync.Mutex
}

func newTelegramController(cfg Config) (*telegramController, error) {
	chatID, err := parsePreviewChatID(cfg.TelegramPreviewChatID)
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

func (tc *telegramController) allowed(chatID int64) bool {
	return chatID == tc.chatID
}

func (tc *telegramController) sendHTML(chatID int64, html string, withRegenerateBtn bool) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if withRegenerateBtn {
		msg.ReplyMarkup = regenerateKeyboard()
	}
	sent, err := tc.bot.Send(msg)
	if err != nil {
		return err
	}
	log.Printf("message_id=%d", sent.MessageID)
	return nil
}

func (tc *telegramController) sendPlain(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := tc.bot.Send(msg); err != nil {
		log.Printf("telegram: %v", err)
	}
}

func (tc *telegramController) startRegenerate(chatID int64) {
	if !tc.allowed(chatID) {
		tc.sendPlain(chatID, "Этот бот доступен только владельцу превью.")
		return
	}
	if !tc.mu.TryLock() {
		tc.sendPlain(chatID, "⏳ Уже собираю дайджест, подождите…")
		return
	}
	go func() {
		defer tc.mu.Unlock()
		tc.sendPlain(chatID, "⏳ Собираю новый дайджест (RSS + Gemini)…")
		if err := runDigest(tc.cfg); err != nil {
			log.Printf("дайджест: %v", err)
			tc.sendPlain(chatID, "❌ Ошибка: "+err.Error())
		}
	}()
}

func (tc *telegramController) handleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		q := update.CallbackQuery
		if q.Data == cbRegenerateDigest && q.Message != nil {
			_, _ = tc.bot.Request(tgbotapi.NewCallback(q.ID, "Запускаю новый дайджест…"))
			tc.startRegenerate(q.Message.Chat.ID)
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
		tc.startRegenerate(chatID)
	}
}

func runTelegramBot(ctx context.Context, cfg Config) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	log.Printf("Telegram: @%s (кнопка и /digest)", tc.bot.Self.UserName)

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

func runTelegramBotUntilSignal(cfg Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		cancel()
	}()

	_ = runTelegramBot(ctx, cfg)
}
