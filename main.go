// Tree Shield VPN — пятничный дайджест: RSS → Gemini → превью в личку.
//
//	go build -o treesheild-newsbot .
//	./treesheild-newsbot -preview   # один раз
//	./treesheild-newsbot            # по CRON_SCHEDULE из .env
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
	once := flag.Bool("preview", false, "собрать дайджест один раз и выйти")
	cronOverride := flag.String("cron", "", "cron (5 полей), перебивает CRON_SCHEDULE")
	flag.Parse()

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	if *cronOverride != "" {
		cfg.CronSchedule = *cronOverride
	}
	if err := cfg.validate(); err != nil {
		log.Fatalf("конфиг: %v", err)
	}

	log.Printf("Tree Shield NewsBot | TZ=%s | модель=%s | cron=%s",
		cfg.Timezone, cfg.GeminiModel, cfg.CronSchedule)

	if *once {
		if err := runDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
		return
	}

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

func startScheduler(cfg Config) (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(cfg.Timezone))
	if err != nil {
		return nil, err
	}

	_, err = scheduler.NewJob(
		gocron.CronJob(cfg.CronSchedule, false),
		gocron.NewTask(func() {
			if err := runDigest(cfg); err != nil {
				log.Printf("ошибка дайджеста: %v", err)
			}
		}),
		gocron.WithName("friday-digest"),
	)
	if err != nil {
		return nil, fmt.Errorf("cron %q: %w", cfg.CronSchedule, err)
	}

	scheduler.Start()
	log.Printf("Планировщик: cron=%q (%s) → личка %s. Ctrl+C для выхода.",
		cfg.CronSchedule, cfg.Timezone, cfg.TelegramPreviewChatID)
	return scheduler, nil
}

func runDigest(cfg Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	now := time.Now().In(cfg.Timezone)
	log.Printf("Сбор новостей за неделю (с %s)…", now.Add(-7*24*time.Hour).Format("02.01.2006"))

	articles, err := fetchWeeklyArticles(ctx, now)
	if err != nil {
		return err
	}
	log.Printf("Отобрано статей: %d", len(articles))

	pool := articlesForPrompt(articles)
	log.Printf("Запрос к Gemini (~%d симв., %d статей)…", len(buildNewsDigestPrompt(articles, 0)), len(pool))

	newsHTML, err := generateDigest(ctx, cfg, articles)
	if err != nil {
		return err
	}

	newsHTML = ensureNewsLinks(newsHTML, articles)
	if err := validateNewsLinks(newsHTML); err != nil {
		return fmt.Errorf("ссылки в заголовках: %w", err)
	}

	log.Printf("Отправка превью в чат %s…", cfg.TelegramPreviewChatID)
	return publishPreview(cfg, assembleDigest(newsHTML))
}
