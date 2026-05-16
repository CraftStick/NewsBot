package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func publishToChannel(cfg Config, htmlText string) error {
	return publishDigest(cfg, cfg.TelegramChannelID, "TELEGRAM_CHANNEL_ID", htmlText, false)
}

func publishPreviewToBot(cfg Config, htmlText string) error {
	const label = "<b>🧪 Тест дайджеста</b> <i>(превью, в канал не публикуется)</i>\n\n"
	return publishDigest(cfg, cfg.TelegramPreviewChatID, "TELEGRAM_PREVIEW_CHAT_ID", label+htmlText, true)
}

func publishDigest(cfg Config, destination, envName, htmlText string, isPreview bool) error {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("telegram bot: %w", err)
	}

	target, err := parseChatTarget(destination, envName)
	if err != nil {
		return err
	}

	var msg tgbotapi.MessageConfig
	if target.Username != "" && !isPreview {
		msg = tgbotapi.NewMessageToChannel(target.Username, htmlText)
	} else {
		msg = tgbotapi.NewMessage(target.ChatID, htmlText)
	}
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	sent, err := bot.Send(msg)
	if err != nil {
		return fmt.Errorf("отправка в Telegram: %w", err)
	}
	log.Printf("Отправлено в Telegram: message_id=%d", sent.MessageID)
	return nil
}
