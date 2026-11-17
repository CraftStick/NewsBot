package news

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"

	"github.com/mmcdole/gofeed"
)

// miniFeeds — источники мини-постов.
var miniFeeds = []struct{ Name, URL string }{
	{"Код Дурова", "https://kod.ru/rss"},
	{"Gram News", "https://gramnews.org/ru/rss"},
}

// FetchMini — свежие записи источников мини-постов, новые первыми.
// Упавшая лента не мешает остальным; ошибка — только если не ответили все.
func FetchMini(ctx context.Context) ([]Article, error) {
	parser := gofeed.NewParser()
	parser.Client = httpClient
	parser.UserAgent = "TreeShieldNewsBot/1.0 (+https://t.me/treeshield)"
	var out []Article
	var errs []error
	for _, src := range miniFeeds {
		feed, err := fetchFeedWithRetry(ctx, parser, src.URL)
		if err != nil {
			log.Printf("мини-посты: %s: %v", src.Name, err)
			errs = append(errs, err)
			continue
		}
		for _, it := range feed.Items {
			if it.Link == "" || strings.TrimSpace(it.Title) == "" || it.PublishedParsed == nil {
				continue
			}
			out = append(out, Article{Source: src.Name, Title: strings.TrimSpace(it.Title),
				Link: it.Link, Summary: miniSummary(it), PublishedAt: *it.PublishedParsed})
		}
	}
	if len(errs) == len(miniFeeds) {
		return nil, errors.Join(errs...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}

// miniSummary — описание целиком; в подпись под фото влезает ~1000 знаков, итог ужмёт app.
func miniSummary(it *gofeed.Item) string {
	d := it.Description
	if d == "" {
		d = it.Content
	}
	return truncate(StripHTML(d), 700)
}
