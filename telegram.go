package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func publishToChannel(cfg Config, htmlText string) error {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("telegram bot: %w", err)
	}

	target, err := parseChannelTarget(cfg.TelegramChannelID)
	if err != nil {
		return err
	}

	mediaMsg, err := sendPreviewMedia(bot, target, cfg.MediaType)
	if err != nil {
		return fmt.Errorf("отправка превью: %w", err)
	}

	var textMsg tgbotapi.MessageConfig
	if target.Username != "" {
		textMsg = tgbotapi.NewMessageToChannel(target.Username, htmlText)
	} else {
		textMsg = tgbotapi.NewMessage(target.ChatID, htmlText)
	}
	textMsg.ParseMode = tgbotapi.ModeHTML
	textMsg.ReplyToMessageID = mediaMsg.MessageID
	textMsg.DisableWebPagePreview = true

	sent, err := bot.Send(textMsg)
	if err != nil {
		return fmt.Errorf("отправка текста: %w", err)
	}

	log.Printf("Опубликовано: media_msg=%d text_msg=%d", mediaMsg.MessageID, sent.MessageID)
	return nil
}

func sendPreviewMedia(bot *tgbotapi.BotAPI, target channelTarget, mediaType string) (tgbotapi.Message, error) {
	switch mediaType {
	case "sticker":
		if _, err := os.Stat(PreviewStickerPath); err != nil {
			return tgbotapi.Message{}, fmt.Errorf("стикер не найден (%s): %w", PreviewStickerPath, err)
		}
		msg := tgbotapi.NewSticker(target.ChatID, tgbotapi.FilePath(PreviewStickerPath))
		applyChannelTarget(&msg.BaseChat, target)
		return bot.Send(msg)

	case "animation":
		if _, err := os.Stat(PreviewAnimationPath); err != nil {
			return tgbotapi.Message{}, fmt.Errorf("анимация не найдена (%s): %w", PreviewAnimationPath, err)
		}
		msg := tgbotapi.NewAnimation(target.ChatID, tgbotapi.FilePath(PreviewAnimationPath))
		applyChannelTarget(&msg.BaseChat, target)
		return bot.Send(msg)

	default:
		if _, err := os.Stat(PreviewImagePath); err != nil {
			return tgbotapi.Message{}, fmt.Errorf("картинка не найдена (%s): положите превью в assets/: %w", PreviewImagePath, err)
		}
		var msg tgbotapi.PhotoConfig
		if target.Username != "" {
			msg = tgbotapi.NewPhotoToChannel(target.Username, tgbotapi.FilePath(PreviewImagePath))
		} else {
			msg = tgbotapi.NewPhoto(target.ChatID, tgbotapi.FilePath(PreviewImagePath))
		}
		msg.Caption = "Пятничный дайджест Tree Shield VPN 🌲"
		return bot.Send(msg)
	}
}

func applyChannelTarget(chat *tgbotapi.BaseChat, target channelTarget) {
	if target.Username != "" {
		chat.ChannelUsername = target.Username
		chat.ChatID = 0
	}
}

func escapeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
