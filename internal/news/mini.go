package news

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"unicode"

	"github.com/mmcdole/gofeed"
)

// miniFeeds — источники мини-постов.
var miniFeeds = []struct {
	Name, URL string
	All       bool // без фильтра по теме
}{
	{"Код Дурова", "https://kod.ru/rss", false},
	{"Gram News", "https://gramnews.org/ru/rss", true},
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
			title, sum := cleanMiniText(it.Title, miniSummary(it))
			// Только по теме канала (блокировки, приватность, ИБ), как в дайджесте.
			rel := articleRelevance(title, sum)
			if !src.All && rel < relevanceTopic {
				continue
			}
			out = append(out, Article{Source: src.Name, Title: title, Link: it.Link,
				Summary: sum, PublishedAt: *it.PublishedParsed, Relevance: rel})
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

// cleanMiniText: у постов Telegram-каналов заголовок равен тексту и начинается со
// своего эмодзи — убираем его (мы ставим свой) и не повторяем заголовок в описании.
func cleanMiniText(title, sum string) (string, string) {
	lead := func(s string) string {
		return strings.TrimSpace(strings.TrimLeftFunc(s, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("«\"(", r)
		}))
	}
	title, sum = lead(strings.TrimSpace(title)), lead(sum)
	ft, fs := fold(title), fold(sum)
	switch {
	case strings.HasPrefix(fs, ft) && !strings.HasSuffix(title, "…") || strings.HasPrefix(ft, fs):
		sum = ""
	case strings.HasSuffix(title, "…") && strings.HasPrefix(fs, ft):
		title, sum = firstSentence(sum) // заголовок у Gram News — обрезанный текст поста: первое предложение в заголовок, остальное под него
	}
	return title, sum
}

// fold оставляет только буквы и цифры: «…» и «*» не мешают сравнить заголовок с текстом.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

// firstSentence делит текст по первому концу предложения; без него весь текст идёт заголовком.
func firstSentence(s string) (head, tail string) {
	for i, r := range s {
		if (r == '.' || r == '!' || r == '?') && i >= 20 && i+1 < len(s) && s[i+1] == ' ' {
			return s[:i+1], strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}
