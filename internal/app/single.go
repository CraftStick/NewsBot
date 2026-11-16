package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
	"treesheild-newsbot/internal/gemini"
	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/store"
	"treesheild-newsbot/internal/telegram"
)

// RunSingle собирает один пост из свежего и ещё не опубликованного и шлёт его
// в личку на проверку.
func RunSingle(cfg config.Config) (err error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	now := time.Now().In(cfg.Timezone)
	articles, failed, err := news.FetchWeeklyArticles(ctx, now)
	run.FeedsFail = failed
	if err != nil {
		return err
	}
	history := digest.ReadHistory(now)
	articles = digest.DropPublished(articles, history)
	if len(articles) == 0 {
		return fmt.Errorf("нет свежих неопубликованных статей")
	}
	run.Candidates = articles

	post, err := gemini.GenerateSingle(ctx, cfg, articles, digest.HistoryTitles(history))
	if err != nil {
		return err
	}
	post = digest.EnsureNewsLinks(post, articles)
	if err := digest.ValidateSingleLink(post); err != nil {
		return err
	}
	run.DigestHTML = post
	if err := telegram.PublishSinglePreview(cfg, post); err != nil {
		return err
	}
	if err := digest.RecordHistory(post, now); err != nil {
		log.Printf("не удалось запомнить тему поста: %v", err)
	}
	return nil
}
