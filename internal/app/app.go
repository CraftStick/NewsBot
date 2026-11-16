// Package app собирает дайджест целиком (RSS → Gemini → Telegram) и
// запускает его по расписанию.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
	"treesheild-newsbot/internal/gemini"
	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/store"
	"treesheild-newsbot/internal/telegram"
)

func RunAfter(cfg config.Config, delay time.Duration) {
	log.Printf("Дайджест в личку через %s… (Ctrl+C — отмена)", delay)
	timer := time.NewTimer(delay)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-timer.C:
		if err := RunDigest(cfg); err != nil {
			log.Fatalf("дайджест: %v", err)
		}
	case <-sig:
		if !timer.Stop() {
			<-timer.C
		}
		log.Println("отменено")
	}
}

func StartScheduler(cfg config.Config) (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(cfg.Timezone))
	if err != nil {
		return nil, err
	}

	_, err = scheduler.NewJob(
		gocron.CronJob(cfg.CronSchedule, false),
		gocron.NewTask(func() {
			if err := RunDigest(cfg); err != nil {
				log.Printf("ошибка дайджеста: %v", err)
				telegram.NotifyFailure(cfg, err)
			}
		}),
		gocron.WithName("friday-digest"),
	)
	if err != nil {
		return nil, fmt.Errorf("cron %q: %w", cfg.CronSchedule, err)
	}

	scheduler.Start()
	log.Printf("Планировщик: дайджест cron=%q (%s) → личка %s. Команда /digest в боте. Ctrl+C — выход.",
		cfg.CronSchedule, cfg.Timezone, cfg.TelegramPreviewChatID)
	return scheduler, nil
}

func RunDigest(cfg config.Config) (err error) {
	run := store.Run{StartedAt: time.Now()}
	defer func() {
		run.Duration = time.Since(run.StartedAt)
		if err != nil {
			run.Err = err.Error()
		}
		if serr := store.Save(store.Path(), run); serr != nil {
			log.Printf("не удалось записать журнал запуска: %v", serr)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	now := time.Now().In(cfg.Timezone)
	log.Printf("Сбор новостей за неделю (с %s)…", now.Add(-7*24*time.Hour).Format("02.01.2006"))

	articles, failed, err := news.FetchWeeklyArticles(ctx, now)
	run.FeedsFail = failed
	if 2*len(failed) >= len(news.FeedNames()) {
		telegram.NotifyWarning(cfg, fmt.Sprintf("⚠️ Не ответили %d лент: %s", len(failed), strings.Join(failed, ", ")))
	}
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

	history := digest.ReadHistory(now)
	if fresh := digest.DropPublished(articles, history); len(fresh) < len(articles) {
		log.Printf("Уже публиковали: отброшено статей %d, осталось %d", len(articles)-len(fresh), len(fresh))
		articles = fresh
	}

	articles = news.FilterOnTopic(articles, 12)

	// kod.ru и Rozetked — основные источники, остальные ленты — резерв.
	sort.SliceStable(articles, func(i, j int) bool { return isMain(articles[i]) && !isMain(articles[j]) })
	run.Candidates = articles
	pool := news.ArticlesForPrompt(articles)
	log.Printf("Запрос к Gemini (~%d симв., %d статей)…", len(news.BuildDigestPrompt(articles, 0)), len(pool))

	// Gemini иногда выдаёт пункт, к которому не находится статья: одна повторная попытка.
	var newsHTML string
	for attempt := 1; ; attempt++ {
		newsHTML, err = gemini.GenerateDigest(ctx, cfg, articles, digest.HistoryTitles(history))
		if err != nil {
			return err
		}
		newsHTML = digest.EnsureNewsLinks(newsHTML, articles)
		if err = digest.ValidateNewsLinks(newsHTML); err == nil {
			break
		}
		if attempt == 2 {
			return fmt.Errorf("ссылки в заголовках: %w", err)
		}
		log.Printf("попытка %d: %v — генерирую заново", attempt, err)
	}

	run.DigestHTML = newsHTML
	log.Printf("Отправка превью в чат %s…", cfg.TelegramPreviewChatID)
	if err := telegram.PublishPreview(cfg, newsHTML, digestPhoto(ctx, newsHTML, articles)); err != nil {
		return err
	}
	if err := recordDigestSent(now); err != nil {
		log.Printf("не удалось записать отметку отправки: %v", err)
	}
	if cfg.PublishCmd == "" { // с кнопкой темы запоминаются при публикации
		if err := digest.RecordHistory(newsHTML, now); err != nil {
			log.Printf("не удалось запомнить темы дайджеста: %v", err)
		}
	}
	return nil
}

func isKod(a news.Article) bool { return strings.Contains(a.Link, "kod.ru") }

func isMain(a news.Article) bool { return isKod(a) || strings.Contains(a.Link, "rozetked.me") }

// digestPhoto — обложка дайджеста: картинка первой статьи kod.ru из дайджеста;
// nil — тогда стандартная (чужие источники не берём). Порядок пунктов = важность.
func digestPhoto(ctx context.Context, html string, articles []news.Article) []byte {
	for _, a := range articles {
		if isKod(a) && strings.Contains(html, a.Link) {
			if b := news.DownloadImage(ctx, news.ImageFor(ctx, a)); b != nil {
				return b
			}
		}
	}
	return nil
}
