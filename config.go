package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Пути к локальным медиа для первого сообщения в канале.
const (
	PreviewImagePath     = "assets/digest_preview.jpg"
	PreviewStickerPath   = "assets/digest_sticker.tgs"
	PreviewAnimationPath = "assets/digest_preview.webm"
)

// systemPrompt — короткая инструкция (экономия токенов). Шапка/подвал — в format.go.
const systemPrompt = `Редактор IT-дайджеста. Из ленты за 7 дней выбери 5 главных тем: VPN, блокировки, приватность, ИБ.
Ответ — только 5 блоков (между блоками пустая строка):
<b>N. Заголовок</b>
2–3 предложения: факт, кого касается, последствия.
Запрещено: emoji, шапка, прощание, реклама Tree Shield, юмор, markdown, ссылки. Тег <b> — только в строке заголовка пункта.`

type Config struct {
	TelegramToken      string
	TelegramChannelID  string
	GeminiAPIKey       string
	GeminiModel        string
	Timezone           *time.Location
	MediaType string
}

func LoadConfig() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		TelegramToken:       strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChannelID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHANNEL_ID")),
		GeminiAPIKey:        strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:         strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		MediaType: strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_TYPE"))),
	}

	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.5-flash"
	}
	if cfg.MediaType == "" {
		cfg.MediaType = "photo"
	}
	tzName := strings.TrimSpace(os.Getenv("TZ"))
	if tzName == "" {
		tzName = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return cfg, fmt.Errorf("неверный TZ %q: %w", tzName, err)
	}
	cfg.Timezone = loc

	if cfg.TelegramToken == "" {
		return cfg, fmt.Errorf("TELEGRAM_BOT_TOKEN не задан")
	}
	if cfg.TelegramChannelID == "" {
		return cfg, fmt.Errorf("TELEGRAM_CHANNEL_ID не задан")
	}
	if cfg.GeminiAPIKey == "" {
		return cfg, fmt.Errorf("GEMINI_API_KEY не задан")
	}

	return cfg, nil
}

// channelTarget — куда публиковать (числовой ID или @username).
type channelTarget struct {
	ChatID   int64
	Username string // с префиксом @
}

func parseChannelTarget(raw string) (channelTarget, error) {
	if strings.HasPrefix(raw, "@") {
		return channelTarget{Username: raw}, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return channelTarget{}, fmt.Errorf("TELEGRAM_CHANNEL_ID: ожидается @channel или -100…: %w", err)
	}
	return channelTarget{ChatID: id}, nil
}
