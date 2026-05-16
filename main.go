// Tree Shield VPN — автономный бот пятничного дайджеста.
//
// Сборка: go build -o treesheild-newsbot .
// Превью в чат с ботом: ./treesheild-newsbot -preview
// Публикация в канал:  ./treesheild-newsbot -run-once
// Планировщик:         ./treesheild-newsbot
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func main() {
	preview := flag.Bool("preview", false, "собрать дайджест и отправить вам в личку с ботом")
	runOnce := flag.Bool("run-once", false, "сразу собрать и опубликовать дайджест в канал")
	flag.Parse()

	if *preview && *runOnce {
		log.Fatal("укажите только один флаг: -preview или -run-once")
	}

	cfg, err := LoadConfig(!*preview)
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	if *preview {
		if err := cfg.validatePreview(); err != nil {
			log.Fatalf("конфиг: %v", err)
		}
	}

	log.Printf("Tree Shield NewsBot | TZ=%s | модель=%s",
		cfg.Timezone, cfg.GeminiModel)

	switch {
	case *preview:
		if err := runDigest(cfg, true); err != nil {
			log.Fatalf("превью: %v", err)
		}
	case *runOnce:
		if err := runDigest(cfg, false); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
	default:
		scheduler, err := startScheduler(cfg)
		if err != nil {
			log.Fatalf("планировщик: %v", err)
		}
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("остановка…")
		_ = scheduler.Shutdown()
	}
}

func startScheduler(cfg Config) (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(cfg.Timezone))
	if err != nil {
		return nil, err
	}

	_, err = scheduler.NewJob(
		gocron.CronJob("0 18 * * 5", false),
		gocron.NewTask(func() {
			if err := runDigest(cfg, false); err != nil {
				log.Printf("ошибка дайджеста: %v", err)
			}
		}),
		gocron.WithName("friday-digest"),
	)
	if err != nil {
		return nil, err
	}

	scheduler.Start()
	log.Printf("Планировщик запущен: пятница 18:00 (%s). Ctrl+C для выхода.", cfg.Timezone)
	return scheduler, nil
}

func runDigest(cfg Config, preview bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	now := time.Now().In(cfg.Timezone)
	log.Printf("Сбор новостей за неделю (с %s)…", now.Add(-7*24*time.Hour).Format("02.01.2006"))

	articles, err := fetchWeeklyArticles(ctx, now)
	if err != nil {
		return err
	}
	log.Printf("Отобрано статей: %d", len(articles))

	prompt := buildNewsDigestPrompt(articles)
	log.Printf("Запрос к Gemini (~%d симв., %d статей в контексте)…", len(prompt), len(articlesForPrompt(articles)))

	newsHTML, err := generateDigest(ctx, cfg, prompt)
	if err != nil {
		return err
	}

	// Ссылки ищем по всей ленте (95+), не только по 32 статьям в промпте.
	newsHTML = ensureNewsLinks(newsHTML, articles)
	if err := validateNewsLinks(newsHTML); err != nil {
		return fmt.Errorf("ссылки в заголовках: %w", err)
	}

	html := assembleDigest(newsHTML)

	if preview {
		log.Printf("Отправка превью в чат %s…", cfg.TelegramPreviewChatID)
		return publishPreviewToBot(cfg, html)
	}

	log.Printf("Публикация в канал…")
	return publishToChannel(cfg, html)
}
