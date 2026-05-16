package main

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func connectTelegram(token string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN неверный — скопируйте заново у @BotFather (/token), без кавычек в .env")
		}
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN: %w", err)
	}
	log.Printf("Telegram: @%s", bot.Self.UserName)
	return bot, nil
}

func (cfg Config) validateTelegram() error {
	if cfg.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не задан")
	}
	_, err := connectTelegram(cfg.TelegramToken)
	return err
}

func (cfg Config) validatePreview() error {
	if err := cfg.validateTelegram(); err != nil {
		return err
	}
	if cfg.TelegramPreviewChatID == "" {
		return fmt.Errorf(`TELEGRAM_PREVIEW_CHAT_ID не задан — напишите боту /start, затем getUpdates:
https://api.telegram.org/bot<ТОКЕН>/getUpdates`)
	}
	return nil
}

func (cfg Config) validateChannel() error {
	if err := cfg.validateTelegram(); err != nil {
		return err
	}
	if cfg.TelegramChannelID == "" {
		return fmt.Errorf("TELEGRAM_CHANNEL_ID не задан")
	}
	return nil
}

func publishToChannel(cfg Config, htmlText string) error {
	return publishDigest(cfg, cfg.TelegramChannelID, "TELEGRAM_CHANNEL_ID", htmlText, false)
}

func publishPreviewToBot(cfg Config, htmlText string) error {
	const label = "<b>🧪 Тест дайджеста</b> <i>(превью, в канал не публикуется)</i>\n\n"
	return publishDigest(cfg, cfg.TelegramPreviewChatID, "TELEGRAM_PREVIEW_CHAT_ID", label+htmlText, true)
}

func publishDigest(cfg Config, destination, envName, htmlText string, isPreview bool) error {
	if len([]rune(htmlText)) > telegramMaxMessage {
		return fmt.Errorf("сообщение слишком длинное для Telegram (%d симв., лимит %d)",
			len([]rune(htmlText)), telegramMaxMessage)
	}

	bot, err := connectTelegram(cfg.TelegramToken)
	if err != nil {
		return err
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
