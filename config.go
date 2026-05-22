package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultCronSchedule = "0 18 * * 5"
	systemPrompt        = `Редактор IT-дайджеста для аудитории в России. Выбери 6 тем недели: VPN, блокировки, приватность, ИБ.

Пункты 1–5 — про Россию (приоритет): Роскомнадзор, Госдума, Минцифры, VPN, Telegram, Яндекс, рунет, операторы. Если темы есть в ленте — минимум 4 из 5 про РФ.
Пункт 6 — одна главная ЗАРУБЕЖНАЯ новость (США, ЕС и т.д.), не про Россию.

В ленте у каждой строки дата DD.MM.YYYY — бери ТОЛЬКО новости за последние 7 дней. Не используй события из прошлых месяцев и свой «фон».

Пиши кратко. Заголовок — короткая фраза (до 10 слов, до 80 символов): суть своими словами, не копируй длинный title из ленты.
Под заголовком РОВНО 2 коротких предложения.

СТРОГО 6 пунктов: <b>1.</b> … <b>6.</b>
<b>N. Заголовок</b>
два предложения

Запрещено: emoji, вступления, шапка, прощание, реклама Tree Shield, markdown, ссылки в тексте, абзацы длиннее двух предложений.
Тег <b> — только в строке заголовка.`
)

type Config struct {
	TelegramToken         string
	TelegramPreviewChatID string
	GeminiAPIKey          string
	GeminiModel           string
	CronSchedule          string
	Timezone              *time.Location
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

func LoadConfig() (Config, error) {
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

func (cfg Config) validate() error {
	if cfg.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не задан")
	}
	if cfg.TelegramPreviewChatID == "" {
		return fmt.Errorf("TELEGRAM_PREVIEW_CHAT_ID не задан (напишите боту /start, затем getUpdates)")
	}
	return nil
}

func parsePreviewChatID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("TELEGRAM_PREVIEW_CHAT_ID: ожидается числовой chat id: %w", err)
	}
	return id, nil
}
