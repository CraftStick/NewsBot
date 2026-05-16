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

// loadEnv читает .env: ENV_FILE → рядом с бинарником → текущая папка.
func loadEnv() {
	if f := strings.TrimSpace(os.Getenv("ENV_FILE")); f != "" {
		_ = godotenv.Load(f)
		return
	}
	var paths []string
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	paths = append(paths, ".env")
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		_ = godotenv.Load(p)
		return
	}
}

// systemPrompt — тон «по-человечески»; шапка и прощание — в format.go.
const systemPrompt = `Редактор IT-дайджеста для аудитории в России. Выбери 6 тем недели: VPN, блокировки, приватность, ИБ.

Пункты 1–5 — про Россию (приоритет): Роскомнадзор, Госдума, Минцифры, VPN, Telegram, Яндекс, рунет, операторы. Если темы есть в ленте — минимум 4 из 5 про РФ.
Пункт 6 — одна главная ЗАРУБЕЖНАЯ новость (США, ЕС и т.д.), не про Россию.

Пиши кратко. Заголовок — короткая фраза (до 10 слов, до 80 символов): суть своими словами, не копируй длинный title из ленты.
Под заголовком РОВНО 2 коротких предложения.

СТРОГО 6 пунктов: <b>1.</b> … <b>6.</b>
<b>N. Заголовок</b>
два предложения

Запрещено: emoji, вступления, шапка, прощание, реклама Tree Shield, markdown, ссылки в тексте, абзацы длиннее двух предложений.
Тег <b> — только в строке заголовка.`

const defaultCronSchedule = "0 18 * * 5" // пятница 18:00

// DigestMode — куда слать дайджест по расписанию (флаги -run-once / -run-in всегда в канал).
type DigestMode string

const (
	DigestModePreview DigestMode = "preview" // только личка (по умолчанию)
	DigestModeChannel DigestMode = "channel" // автопост в TELEGRAM_CHANNEL_ID
)

type Config struct {
	TelegramToken         string
	TelegramChannelID     string
	TelegramPreviewChatID string
	GeminiAPIKey          string
	GeminiModel           string
	CronSchedule          string
	DigestMode            DigestMode
	Timezone              *time.Location
}

func (cfg Config) deliversPreview() bool {
	return cfg.DigestMode != DigestModeChannel
}

func parseDigestMode(raw string) DigestMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "channel", "канал":
		return DigestModeChannel
	default:
		return DigestModePreview
	}
}

func LoadConfig() (Config, error) {
	loadEnv()

	cfg := Config{
		TelegramToken:         strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChannelID:     strings.TrimSpace(os.Getenv("TELEGRAM_CHANNEL_ID")),
		TelegramPreviewChatID: strings.TrimSpace(os.Getenv("TELEGRAM_PREVIEW_CHAT_ID")),
		GeminiAPIKey:          strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:           strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		DigestMode:            parseDigestMode(os.Getenv("DIGEST_MODE")),
	}

	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.5-flash"
	}
	cfg.CronSchedule = strings.TrimSpace(os.Getenv("CRON_SCHEDULE"))
	if cfg.CronSchedule == "" {
		cfg.CronSchedule = defaultCronSchedule
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

	if cfg.GeminiAPIKey == "" {
		return cfg, fmt.Errorf("GEMINI_API_KEY не задан")
	}

	return cfg, nil
}

// chatTarget — чат Telegram: личка (положительный id), канал (-100…) или @username.
type chatTarget struct {
	ChatID   int64
	Username string // с префиксом @
}

func parseChatTarget(raw, envName string) (chatTarget, error) {
	if strings.HasPrefix(raw, "@") {
		return chatTarget{Username: raw}, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return chatTarget{}, fmt.Errorf("%s: ожидается числовой id или @username: %w", envName, err)
	}
	return chatTarget{ChatID: id}, nil
}

