// Tree Shield VPN — автономный бот пятничного дайджеста.
//
// Сборка: go build -o treesheild-newsbot .
// Разовый запуск (без ожидания пятницы): ./treesheild-newsbot -run-once
// Планировщик: каждую пятницу в 18:00 по TZ из .env (по умолчанию Москва).
//
// Шапка, подзаголовок и прощание — фиксированный HTML в format.go.
// Gemini генерирует только блок из 5 новостей.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func main() {
	runOnce := flag.Bool("run-once", false, "сразу собрать и опубликовать дайджест (для проверки)")
	flag.Parse()

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}

	log.Printf("Tree Shield NewsBot | TZ=%s | модель=%s | медиа=%s",
		cfg.Timezone, cfg.GeminiModel, cfg.MediaType)

	if *runOnce {
		if err := runDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
		return
	}

	if err := startScheduler(cfg); err != nil {
		log.Fatalf("планировщик: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("остановка")
}

func startScheduler(cfg Config) error {
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(cfg.Timezone))
	if err != nil {
		return err
	}

	_, err = scheduler.NewJob(
		gocron.CronJob("0 18 * * 5", false),
		gocron.NewTask(func() {
			if err := runDigest(cfg); err != nil {
				log.Printf("ошибка дайджеста: %v", err)
			}
		}),
		gocron.WithName("friday-digest"),
	)
	if err != nil {
		return err
	}

	scheduler.Start()
	log.Printf("Планировщик запущен: пятница 18:00 (%s). Жду…", cfg.Timezone)
	select {}
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

	prompt := buildNewsDigestPrompt(articles)
	log.Printf("Запрос к Gemini (~%d симв., %d статей в контексте)…", len(prompt), len(articlesForPrompt(articles)))

	newsHTML, err := generateDigest(ctx, cfg, prompt)
	if err != nil {
		return err
	}

	html := assembleDigest(newsHTML)

	log.Printf("Публикация в Telegram: %s", escapeForLog(html))

	return publishToChannel(cfg, html)
}
