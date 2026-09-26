// Package config читает настройки бота из окружения и .env.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultCronSchedule = "0 18 * * 5"

type Config struct {
	TelegramToken         string
	TelegramPreviewChatID string
	GeminiAPIKey          string
	GeminiModel           string
	CronSchedule          string
	Timezone              *time.Location
	PhotoEnabled          bool
}

func loadEnv() {
	if f := strings.TrimSpace(os.Getenv("ENV_FILE")); f != "" {
		_ = godotenv.Load(f)
		return
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), ".env")
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
	}
	_ = godotenv.Load(".env")
}

func Load() (Config, error) {
	loadEnv()

	cfg := Config{
		TelegramToken:         strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramPreviewChatID: strings.TrimSpace(os.Getenv("TELEGRAM_PREVIEW_CHAT_ID")),
		GeminiAPIKey:          strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:           strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		CronSchedule:          strings.TrimSpace(os.Getenv("CRON_SCHEDULE")),
	}
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.5-flash"
	}
	if cfg.CronSchedule == "" {
		cfg.CronSchedule = defaultCronSchedule
	}

	// Картинка-шапка включена по умолчанию; выключается MEDIA_TYPE=none/off/text.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_TYPE"))) {
	case "none", "off", "text", "no":
		cfg.PhotoEnabled = false
	default:
		cfg.PhotoEnabled = true
	}

	tz := strings.TrimSpace(os.Getenv("TZ"))
	if tz == "" {
		tz = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return cfg, fmt.Errorf("неверный TZ %q: %w", tz, err)
	}
	cfg.Timezone = loc

	if cfg.GeminiAPIKey == "" {
		return cfg, fmt.Errorf("GEMINI_API_KEY не задан")
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не задан")
	}
	if cfg.TelegramPreviewChatID == "" {
		return fmt.Errorf("TELEGRAM_PREVIEW_CHAT_ID не задан (напишите боту /start, затем getUpdates)")
	}
	return nil
}

func ParsePreviewChatID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("TELEGRAM_PREVIEW_CHAT_ID: ожидается числовой chat id: %w", err)
	}
	return id, nil
}
