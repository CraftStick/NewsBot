// Tree Shield VPN — автономный бот пятничного дайджеста.
//
// Сборка: go build -o treesheild-newsbot .
// Превью в личку (копировать в канал): ./treesheild-newsbot -preview
// Сразу в канал:      ./treesheild-newsbot -run-once
// В канал через 1м:   ./treesheild-newsbot -run-in 1m
// Планировщик:        ./treesheild-newsbot  (CRON_SCHEDULE в .env)
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
	runIn := flag.Duration("run-in", 0, "один раз опубликовать в канал через указанное время (например 1m, 30s)")
	cronOverride := flag.String("cron", "", "cron-расписание (5 полей), перебивает CRON_SCHEDULE из .env")
	flag.Parse()

	modes := 0
	if *preview {
		modes++
	}
	if *runOnce {
		modes++
	}
	if *runIn > 0 {
		modes++
	}
	if modes > 1 {
		log.Fatal("укажите только один режим: -preview, -run-once или -run-in")
	}

	cfg, err := LoadConfig(!*preview)
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	if *cronOverride != "" {
		cfg.CronSchedule = *cronOverride
	}

	if *preview {
		if err := cfg.validatePreview(); err != nil {
			log.Fatalf("конфиг: %v", err)
		}
	}

	log.Printf("Tree Shield NewsBot | TZ=%s | модель=%s | cron=%s",
		cfg.Timezone, cfg.GeminiModel, cfg.CronSchedule)

	switch {
	case *preview:
		if err := runDigest(cfg, true); err != nil {
			log.Fatalf("превью: %v", err)
		}
	case *runOnce:
		if err := cfg.validateChannel(); err != nil {
			log.Fatalf("конфиг: %v", err)
		}
		if err := runDigest(cfg, false); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
	case *runIn > 0:
		if err := cfg.validateChannel(); err != nil {
			log.Fatalf("конфиг: %v", err)
		}
		runChannelAfter(cfg, *runIn)
	default:
		if err := cfg.validateChannel(); err != nil {
			log.Fatalf("конфиг: %v", err)
		}
		scheduler, err := startScheduler(cfg)
		if err != nil {
			log.Fatalf("планировщик: %v", err)
		}
		waitForShutdown(scheduler)
	}
}

func runChannelAfter(cfg Config, delay time.Duration) {
	log.Printf("Публикация в канал %s через %s… (Ctrl+C — отмена)", cfg.TelegramChannelID, delay)
	timer := time.NewTimer(delay)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-timer.C:
		if err := runDigest(cfg, false); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
	case <-sig:
		if !timer.Stop() {
			<-timer.C
		}
		log.Println("отменено")
	}
}

func waitForShutdown(scheduler gocron.Scheduler) {
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
			if err := runDigest(cfg, false); err != nil {
				log.Printf("ошибка дайджеста: %v", err)
			}
		}),
		gocron.WithName("digest"),
	)
	if err != nil {
		return nil, fmt.Errorf("cron %q: %w", cfg.CronSchedule, err)
	}

	scheduler.Start()
	log.Printf("Планировщик: cron=%q (%s). Ctrl+C для выхода.", cfg.CronSchedule, cfg.Timezone)
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
	inPrompt := len(articlesForPrompt(articles))
	log.Printf("Запрос к Gemini (~%d симв., %d статей в контексте)…", len(prompt), inPrompt)

	newsHTML, err := generateDigest(ctx, cfg, articles)
	if err != nil {
		return err
	}

	newsHTML = ensureNewsLinks(newsHTML, articles)
	if err := validateNewsLinks(newsHTML); err != nil {
		return fmt.Errorf("ссылки в заголовках: %w", err)
	}

	html := assembleDigest(newsHTML)

	if preview {
		log.Printf("Отправка превью в чат %s…", cfg.TelegramPreviewChatID)
		return publishPreviewToBot(cfg, html)
	}

	log.Printf("Публикация в канал %s…", cfg.TelegramChannelID)
	return publishToChannel(cfg, html)
}
