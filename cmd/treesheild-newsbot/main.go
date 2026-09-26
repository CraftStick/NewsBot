// Tree Shield NewsBot — пятничный дайджест: RSS → Gemini → превью в личку.
// https://github.com/CraftStick/NewsBot
//
//	go build -o treesheild-newsbot ./cmd/treesheild-newsbot
//	./treesheild-newsbot -preview      # в личку + кнопка «Другой дайджест»
//	./treesheild-newsbot -in 1m        # в личку через минуту
//	./treesheild-newsbot               # cron + кнопка /digest в фоне
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"treesheild-newsbot/internal/app"
	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/telegram"
)

func main() {
	once := flag.Bool("preview", false, "сразу собрать дайджест и отправить в личку")
	runIn := flag.Duration("in", 0, "через сколько отправить в личку (например 1m, 30s)")
	cronOverride := flag.String("cron", "", "cron (5 полей), перебивает CRON_SCHEDULE")
	flag.Parse()

	if *once && *runIn > 0 {
		log.Fatal("укажите либо -preview, либо -in, не оба")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	if *cronOverride != "" {
		cfg.CronSchedule = *cronOverride
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("конфиг: %v", err)
	}

	log.Printf("Tree Shield NewsBot | TZ=%s | модель=%s | cron=%s",
		cfg.Timezone, cfg.GeminiModel, cfg.CronSchedule)

	regenerate := func() error { return app.RunDigest(cfg) }

	switch {
	case *once:
		if err := app.RunDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
		log.Println("Жду кнопку «Другой дайджест» или /digest. Ctrl+C — выход.")
		telegram.RunUntilSignal(cfg, regenerate)
		return
	case *runIn > 0:
		app.RunAfter(cfg, *runIn)
		return
	}

	scheduler, err := app.StartScheduler(cfg)
	if err != nil {
		log.Fatalf("планировщик: %v", err)
	}

	botCtx, botCancel := context.WithCancel(context.Background())
	defer botCancel()
	go func() {
		if err := telegram.Run(botCtx, cfg, regenerate); err != nil {
			log.Printf("telegram bot: %v", err)
		}
	}()

	// Досылаем пятничный дайджест, если его слот пришёлся на простой (ребут).
	go app.RunCatchUpIfMissed(cfg)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("остановка…")
	botCancel()
	_ = scheduler.Shutdown()
}
