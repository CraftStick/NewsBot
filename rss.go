package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// feedSources — публичные RSS без API-ключей.
var feedSources = []struct {
	Name string
	URL  string
}{
	// ——— Россия: регуляторика, VPN, мессенджеры ———
	{Name: "Google News — Роскомнадзор", URL: "https://news.google.com/rss/search?q=%D0%A0%D0%BE%D1%81%D0%BA%D0%BE%D0%BC%D0%BD%D0%B0%D0%B4%D0%B7%D0%BE%D1%80+VPN+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — Госдума и VPN", URL: "https://news.google.com/rss/search?q=%D0%93%D0%BE%D1%81%D0%B4%D1%83%D0%BC%D0%B0+VPN+%D0%B8%D0%BD%D1%82%D0%B5%D1%80%D0%BD%D0%B5%D1%82&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — Telegram в РФ", URL: "https://news.google.com/rss/search?q=%D0%A2%D0%B5%D0%BB%D0%B5%D0%B3%D1%80%D0%B0%D0%BC+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%A0%D0%BE%D1%81%D1%81%D0%B8%D1%8F&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — Яндекс и интернет", URL: "https://news.google.com/rss/search?q=%D0%AF%D0%BD%D0%B4%D0%B5%D0%BA%D1%81+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%BC%D0%B5%D1%81%D1%81%D0%B5%D0%BD%D0%B4%D0%B6%D0%B5%D1%80&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — Минцифры и рунет", URL: "https://news.google.com/rss/search?q=%D0%9C%D0%B8%D0%BD%D1%86%D0%B8%D1%84%D1%80%D1%8B+VPN+%D1%80%D1%83%D0%BD%D0%B5%D1%82&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Роскомсвобода", URL: "https://roskomsvoboda.org/feed/"},
	{Name: "OpenNet", URL: "https://www.opennet.ru/opennews/opennews_all_utf.rss"},
	{Name: "Meduza", URL: "https://meduza.io/rss/all"},
	{Name: "Lenta.ru — интернет", URL: "https://lenta.ru/rss/news/internet"},

	// ——— Русскоязычные IT ———
	{Name: "Habr — Информационная безопасность", URL: "https://habr.com/ru/rss/hub/infosecurity/"},
	{Name: "Habr — Разработка", URL: "https://habr.com/ru/rss/flows/develop/"},
	{Name: "VC.ru", URL: "https://vc.ru/rss"},
	{Name: "SecurityLab", URL: "https://www.securitylab.ru/_Services/export/rss/"},
	{Name: "Anti-Malware.ru", URL: "https://www.anti-malware.ru/news/feed/"},
	{Name: "CNews", URL: "https://www.cnews.ru/inc/rss/news.xml"},
	{Name: "ComNews (телеком)", URL: "https://www.comnews.ru/rss.xml"},
	{Name: "ixbt.com", URL: "https://www.ixbt.com/export/news.rss"},
	{Name: "4pda", URL: "https://4pda.to/feed/"},
	{Name: "IT-World", URL: "https://www.it-world.ru/rss/"},
	{Name: "Kaspersky — блог", URL: "https://www.kaspersky.ru/blog/feed/"},
	{Name: "Google News — VPN и блокировки (RU)", URL: "https://news.google.com/rss/search?q=VPN+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%BE%D0%B1%D1%85%D0%BE%D0%B4+%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D0%BE%D1%81%D1%82%D1%8C&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — рунет и цензура (RU)", URL: "https://news.google.com/rss/search?q=%D1%80%D1%83%D0%BD%D0%B5%D1%82+%D1%86%D0%B5%D0%BD%D0%B7%D1%83%D1%80%D0%B0+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%BC%D0%B5%D1%81%D1%81%D0%B5%D0%BD%D0%B4%D0%B6%D0%B5%D1%80&hl=ru&gl=RU&ceid=RU:ru"},

	// ——— Международные ———
	{Name: "Reddit r/VPN", URL: "https://www.reddit.com/r/VPN/.rss"},
	{Name: "Reddit r/privacy", URL: "https://www.reddit.com/r/privacy/.rss"},
	{Name: "Reddit r/technology", URL: "https://www.reddit.com/r/technology/.rss"},
	{Name: "Google News — VPN censorship (EN)", URL: "https://news.google.com/rss/search?q=VPN+censorship+blocking+bypass&hl=en-US&gl=US&ceid=US:en"},
}

// Article — нормализованная новость для фильтра и промпта.
type Article struct {
	Source      string
	Title       string
	Link        string
	Summary     string
	PublishedAt time.Time
	RUPriority  int // выше = приоритетнее для дайджеста про РФ
}

// titleKeywords — фильтр по заголовку (регистронезависимо, подстрока).
var titleKeywords = []string{
	"vpn", "блокировк", "обход", "приватность", "рунет",
	"цензур", "запрет", "разблок", "мессенджер", "шифрован",
	"взлом", "кибер", "утечк", "тспу", "роском", "ркн",
	"госдум", "минцифр", "законопроект", "регулятор",
	"telegram", "телеграм", "whatsapp", "яндекс", "сбер",
	"censorship", "privacy", "firewall", "dpi", "proxy", "интернет",
}

// ruBoostKeywords — повышают приоритет статей про Россию в ленте для Gemini.
var ruBoostKeywords = []string{
	"россия", "россий", "рф", "рунет", "москв",
	"роскомнадзор", "ркн", "госдум", "минцифр", "кремл",
	"telegram", "телеграм", "яндекс", "whatsapp", "госуслуг",
	"суверен", "национальн", "оператор", "мегафон", "мтс", "билайн",
}

// Лимиты для Gemini: меньше входа и RSS-обработки — ниже расход токенов.
const (
	maxItemsPerFeed     = 12 // свежих записей с одной ленты
	maxArticlesInPrompt = 32 // в запрос к модели (приоритет РФ + дата)
	maxSummaryRunes     = 100
)

const (
	rssFetchAttempts = 2
	rssRetryDelay    = 2 * time.Second
)

var httpClient = &http.Client{Timeout: 90 * time.Second}

func fetchFeedWithRetry(ctx context.Context, parser *gofeed.Parser, url string) (*gofeed.Feed, error) {
	var lastErr error
	for attempt := 1; attempt <= rssFetchAttempts; attempt++ {
		feed, err := parser.ParseURLWithContext(url, ctx)
		if err == nil {
			return feed, nil
		}
		lastErr = err
		if attempt < rssFetchAttempts && ctx.Err() == nil {
			time.Sleep(rssRetryDelay)
		}
	}
	return nil, lastErr
}

func fetchWeeklyArticles(ctx context.Context, now time.Time) ([]Article, error) {
	since := now.Add(-7 * 24 * time.Hour)
	parser := gofeed.NewParser()
	parser.Client = httpClient
	parser.UserAgent = "TreeShieldNewsBot/1.0 (+https://t.me/treeshield)"

	seen := make(map[string]struct{})
	var out []Article

	for _, src := range feedSources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		feed, err := fetchFeedWithRetry(ctx, parser, src.URL)
		if err != nil {
			log.Printf("RSS %q: пропуск (%v)", src.Name, err)
			continue
		}

		perFeed := 0
		for _, item := range feed.Items {
			if perFeed >= maxItemsPerFeed {
				break
			}
			if item == nil || item.Title == "" {
				continue
			}
			pub := item.PublishedParsed
			if pub == nil {
				pub = item.UpdatedParsed
			}
			if pub == nil || pub.Before(since) {
				continue
			}
			if !titleMatchesKeywords(item.Title) {
				continue
			}

			link := item.Link
			if link == "" {
				link = firstLink(item.Links)
			}
			key := dedupeKey(item.Title, link)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			perFeed++

			title := cleanText(item.Title)
			summary := cleanText(shortSummary(item))
			out = append(out, Article{
				Source:      src.Name,
				Title:       title,
				Link:        link,
				Summary:     summary,
				PublishedAt: pub.In(now.Location()),
				RUPriority:  ruNewsPriority(title, summary),
			})
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("за последние 7 дней не найдено статей по ключевым словам")
	}
	sortArticlesForPrompt(out)
	return out, nil
}

func ruNewsPriority(title, summary string) int {
	text := strings.ToLower(title + " " + summary)
	score := 0
	for _, kw := range ruBoostKeywords {
		if strings.Contains(text, kw) {
			score++
		}
	}
	return score
}

// sortArticlesForPrompt — сначала новости про РФ, внутри группы по дате.
func sortArticlesForPrompt(articles []Article) {
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].RUPriority != articles[j].RUPriority {
			return articles[i].RUPriority > articles[j].RUPriority
		}
		return articles[i].PublishedAt.After(articles[j].PublishedAt)
	})
}

func titleMatchesKeywords(title string) bool {
	lower := strings.ToLower(title)
	for _, kw := range titleKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func dedupeKey(title, link string) string {
	if link != "" {
		return strings.ToLower(link)
	}
	return strings.ToLower(strings.TrimSpace(title))
}

func firstLink(links []string) string {
	for _, l := range links {
		if l != "" {
			return l
		}
	}
	return ""
}

func shortSummary(item *gofeed.Item) string {
	if item.Description != "" {
		return truncate(stripHTML(item.Description), maxSummaryRunes)
	}
	if item.Content != "" {
		return truncate(stripHTML(item.Content), maxSummaryRunes)
	}
	return ""
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "<") && strings.Contains(s, ">") {
		start := strings.Index(s, "<")
		end := strings.Index(s[start:], ">")
		if end < 0 {
			break
		}
		s = s[:start] + " " + s[start+end+1:]
	}
	return strings.Join(strings.Fields(s), " ")
}

func cleanText(s string) string {
	return strings.TrimSpace(stripHTML(s))
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func articlesForPrompt(all []Article) []Article {
	if len(all) <= maxArticlesInPrompt {
		return all
	}
	return all[:maxArticlesInPrompt]
}

// shortSource сокращает длинные названия лент в промпте.
func shortSource(name string) string {
	for _, sep := range []string{" — ", " - "} {
		if i := strings.Index(name, sep); i > 0 {
			return strings.TrimSpace(name[i+len(sep):])
		}
	}
	if len(name) > 18 {
		return name[:18]
	}
	return name
}

func buildNewsDigestPrompt(articles []Article) string {
	articles = articlesForPrompt(articles)
	var b strings.Builder
	b.WriteString("Лента 7д (приоритет — Россия/рунет, ↓новее):\n")
	for i, a := range articles {
		line := fmt.Sprintf("%d.%s|%s|%s|%s",
			i+1,
			a.PublishedAt.Format("02.01"),
			shortSource(a.Source),
			a.Title,
			a.Link,
		)
		if a.Summary != "" {
			line += "|" + a.Summary
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
