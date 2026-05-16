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
	return publishHTML(cfg, cfg.TelegramChannelID, "TELEGRAM_CHANNEL_ID", htmlText, false)
}

// publishPreviewToBot: подсказка + чистый дайджест (удобно скопировать в канал вручную).
func publishPreviewToBot(cfg Config, htmlText string) error {
	bot, err := connectTelegram(cfg.TelegramToken)
	if err != nil {
		return err
	}
	target, err := parseChatTarget(cfg.TelegramPreviewChatID, "TELEGRAM_PREVIEW_CHAT_ID")
	if err != nil {
		return err
	}

	const hint = "<i>Превью готово.</i> Скопируйте <b>следующее</b> сообщение и опубликуйте в канал вручную — так сохранятся анимированные эмодзи."
	if err := sendHTML(bot, target, hint, true); err != nil {
		return err
	}
	if err := sendHTML(bot, target, htmlText, true); err != nil {
		return err
	}
	log.Printf("Превью: 2 сообщения в личку (второе — для копирования в канал)")
	return nil
}

func publishHTML(cfg Config, destination, envName, htmlText string, isPreview bool) error {
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
	return sendHTML(bot, target, htmlText, isPreview)
}

func sendHTML(bot *tgbotapi.BotAPI, target chatTarget, htmlText string, isPreview bool) error {
	if len([]rune(htmlText)) > telegramMaxMessage {
		return fmt.Errorf("сообщение слишком длинное (%d симв.)", len([]rune(htmlText)))
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
