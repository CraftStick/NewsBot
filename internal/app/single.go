package app

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	if cfg.SingleSource != "" {
		var own []news.Article
		for _, a := range articles {
			for _, name := range strings.Split(cfg.SingleSource, ",") {
				if strings.EqualFold(a.Source, strings.TrimSpace(name)) {
					own = append(own, a)
					break
				}
			}
		}
		articles = own
	}
	if len(articles) == 0 {
		return fmt.Errorf("нет свежих неопубликованных статей")
	}
	run.Candidates = articles

	post, err := gemini.GenerateSingle(ctx, cfg, articles, digest.HistoryTitles(history))
	if err != nil {
		return err
	}
	post, err = digest.EnsureSingleLink(post, articles)
	if err != nil {
		return err
	}
	if err := digest.ValidateSingleLink(post); err != nil {
		return err
	}
	run.DigestHTML = post
	if err := telegram.PublishSinglePreview(cfg, post, postImage(ctx, post, articles)); err != nil {
		return err
	}
	if err := digest.RecordHistory(post, now); err != nil {
		log.Printf("не удалось запомнить тему поста: %v", err)
	}
	return nil
}

// postImage — картинка статьи, на которую ссылается пост; пусто, если не нашли.
func postImage(ctx context.Context, post string, articles []news.Article) string {
	if a := linkedArticle(post, articles); a != nil {
		return news.ImageFor(ctx, *a)
	}
	return ""
}

func linkedArticle(post string, articles []news.Article) *news.Article {
	for i, a := range articles {
		if a.Link != "" && strings.Contains(post, a.Link) {
			return &articles[i]
		}
	}
	return nil
}
