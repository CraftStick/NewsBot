// Tree Shield NewsBot — пятничный дайджест: RSS → Gemini → превью в личку.
// https://github.com/CraftStick/NewsBot
//
//	go build -o treesheild-newsbot .
//	./treesheild-newsbot -preview      # в личку + кнопка «Другой дайджест»
//	./treesheild-newsbot -in 1m        # в личку через минуту
//	./treesheild-newsbot               # cron + кнопка /digest в фоне
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
	once := flag.Bool("preview", false, "сразу собрать дайджест и отправить в личку")
	runIn := flag.Duration("in", 0, "через сколько отправить в личку (например 1m, 30s)")
	cronOverride := flag.String("cron", "", "cron (5 полей), перебивает CRON_SCHEDULE")
	flag.Parse()

	if *once && *runIn > 0 {
		log.Fatal("укажите либо -preview, либо -in, не оба")
	}

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

	switch {
	case *once:
		if err := runDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
		log.Println("Жду кнопку «Другой дайджест» или /digest. Ctrl+C — выход.")
		runTelegramBotUntilSignal(cfg)
		return
	case *runIn > 0:
		runAfter(cfg, *runIn)
		return
	}

	scheduler, err := startScheduler(cfg)
	if err != nil {
		log.Fatalf("планировщик: %v", err)
	}

	botCtx, botCancel := context.WithCancel(context.Background())
	defer botCancel()
	go func() {
		if err := runTelegramBot(botCtx, cfg); err != nil {
			log.Printf("telegram bot: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("остановка…")
	botCancel()
	_ = scheduler.Shutdown()
}

func runAfter(cfg Config, delay time.Duration) {
	log.Printf("Дайджест в личку через %s… (Ctrl+C — отмена)", delay)
	timer := time.NewTimer(delay)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-timer.C:
		if err := runDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
	case <-sig:
		if !timer.Stop() {
			<-timer.C
		}
		log.Println("отменено")
	}
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
	log.Printf("Планировщик: cron=%q (%s) → личка %s. Кнопка /digest в боте. Ctrl+C — выход.",
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
	if len(articles) > 0 {
		newest := articles[0].PublishedAt
		oldest := articles[len(articles)-1].PublishedAt
		for _, a := range articles {
			if a.PublishedAt.After(newest) {
				newest = a.PublishedAt
			}
			if a.PublishedAt.Before(oldest) {
				oldest = a.PublishedAt
			}
		}
		log.Printf("Диапазон дат в ленте: %s — %s", oldest.Format("02.01.2006"), newest.Format("02.01.2006"))
	}

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
