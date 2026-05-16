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
	// ——— Русскоязычные ———
	{Name: "Habr — Информационная безопасность", URL: "https://habr.com/ru/rss/hub/infosecurity/"},
	{Name: "Habr — Сети", URL: "https://habr.com/ru/rss/hub/networks/"},
	{Name: "Habr — Администрирование", URL: "https://habr.com/ru/rss/hub/admin/"},
	{Name: "VC.ru", URL: "https://vc.ru/rss"},
	{Name: "SecurityLab", URL: "https://www.securitylab.ru/_Services/export/rss/"},
	{Name: "Anti-Malware.ru", URL: "https://www.anti-malware.ru/news/feed/"},
	{Name: "CNews", URL: "https://www.cnews.ru/inc/rss/news.xml"},
	{Name: "ComNews (телеком)", URL: "https://www.comnews.ru/rss.xml"},
	{Name: "ixbt.com", URL: "https://www.ixbt.com/export/news.rss"},
	{Name: "4pda", URL: "https://4pda.to/feed/"},
	{Name: "IT-World", URL: "https://www.it-world.ru/rss/"},
	{Name: "Kaspersky — блог", URL: "https://www.kaspersky.ru/blog/feed/"},
	{Name: "Positive Technologies", URL: "https://www.ptsecurity.com/ru-ru/research/analytics/feed/"},
	{Name: "OpenNet", URL: "https://www.opennet.ru/opennews/opennews_all_utf.rss"},
	{Name: "Роскомсвобода", URL: "https://roskomsvoboda.org/feed/"},
	{Name: "Lenta.ru — интернет", URL: "https://lenta.ru/rss/news/internet"},
	{Name: "РБК — технологии", URL: "https://rssexport.rbc.ru/rbcnews/technology/20/full.rss"},
	{Name: "TJournal", URL: "https://tjournal.ru/rss/all"},
	{Name: "Google News — VPN и блокировки (RU)", URL: "https://news.google.com/rss/search?q=VPN+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%BE%D0%B1%D1%85%D0%BE%D0%B4+%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D0%BE%D1%81%D1%82%D1%8C&hl=ru&gl=RU&ceid=RU:ru"},
	{Name: "Google News — рунет и цензура (RU)", URL: "https://news.google.com/rss/search?q=%D1%80%D1%83%D0%BD%D0%B5%D1%82+%D1%86%D0%B5%D0%BD%D0%B7%D1%83%D1%80%D0%B0+%D0%B1%D0%BB%D0%BE%D0%BA%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0+%D0%BC%D0%B5%D1%81%D1%81%D0%B5%D0%BD%D0%B4%D0%B6%D0%B5%D1%80&hl=ru&gl=RU&ceid=RU:ru"},

	// ——— Международные ———
	{Name: "Reddit r/VPN", URL: "https://www.reddit.com/r/VPN/.rss"},
	{Name: "Reddit r/privacy", URL: "https://www.reddit.com/r/privacy/.rss"},
	{Name: "Reddit r/technology", URL: "https://www.reddit.com/r/technology/.rss"},
	{Name: "Reddit r/ru", URL: "https://www.reddit.com/r/ru/.rss"},
	{Name: "Google News — VPN censorship (EN)", URL: "https://news.google.com/rss/search?q=VPN+censorship+blocking+bypass&hl=en-US&gl=US&ceid=US:en"},
}

// Article — нормализованная новость для фильтра и промпта.
type Article struct {
	Source      string
	Title       string
	Link        string
	Summary     string
	PublishedAt time.Time
}

// titleKeywords — фильтр по заголовку (регистронезависимо, подстрока).
var titleKeywords = []string{
	"vpn", "блокировк", "обход", "приватность", "рунет",
	"цензур", "запрет", "разблок", "мессенджер", "шифрован",
	"взлом", "кибер", "утечк", "тспу", "роском",
	"censorship", "privacy", "firewall", "dpi", "proxy",
	"telegram", "whatsapp", "интернет",
}

// Лимиты для Gemini: меньше входа и RSS-обработки — ниже расход токенов.
const (
	maxItemsPerFeed     = 12 // свежих записей с одной ленты
	maxArticlesInPrompt = 28 // в запрос к модели (уже отсортированы по дате)
	maxSummaryRunes     = 100
)

var httpClient = &http.Client{Timeout: 45 * time.Second}

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

		feed, err := parser.ParseURLWithContext(src.URL, ctx)
		if err != nil {
			log.Printf("RSS %q: %v", src.Name, err)
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

			out = append(out, Article{
				Source:      src.Name,
				Title:       cleanText(item.Title),
				Link:        link,
				Summary:     cleanText(shortSummary(item)),
				PublishedAt: pub.In(now.Location()),
			})
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("за последние 7 дней не найдено статей по ключевым словам")
	}
	sortArticlesByDate(out)
	return out, nil
}

func sortArticlesByDate(articles []Article) {
	sort.Slice(articles, func(i, j int) bool {
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
	b.WriteString("Лента 7д (↓новее):\n")
	for i, a := range articles {
		line := fmt.Sprintf("%d.%s|%s|%s",
			i+1,
			a.PublishedAt.Format("02.01"),
			shortSource(a.Source),
			a.Title,
		)
		if a.Summary != "" {
			line += "|" + a.Summary
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
