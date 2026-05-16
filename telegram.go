package main

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func publishPreview(cfg Config, htmlText string) error {
	if n := len([]rune(htmlText)); n > telegramMaxMessage {
		return fmt.Errorf("дайджест слишком длинный (%d симв., лимит %d)", n, telegramMaxMessage)
	}

	chatID, err := parsePreviewChatID(cfg.TelegramPreviewChatID)
	if err != nil {
		return err
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			return fmt.Errorf("TELEGRAM_BOT_TOKEN неверный — проверьте .env")
		}
		return fmt.Errorf("telegram: %w", err)
	}
	log.Printf("Telegram: @%s", bot.Self.UserName)

	const hint = "<i>Превью готово.</i> Скопируйте <b>следующее</b> сообщение и опубликуйте в канал."
	if err := sendMessage(bot, chatID, hint); err != nil {
		return err
	}
	if err := sendMessage(bot, chatID, htmlText); err != nil {
		return err
	}
	log.Printf("Превью отправлено (2 сообщения)")
	return nil
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, html string) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	sent, err := bot.Send(msg)
	if err != nil {
		return err
	}
	log.Printf("message_id=%d", sent.MessageID)
	return nil
}
