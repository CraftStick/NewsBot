// Package news собирает статьи за неделю из RSS-лент и отбирает их по теме.
package news

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"
)

// feedSources — публичные RSS без API-ключей.
var feedSources = []struct {
	Name string
	URL  string
}{
	// ——— Россия: регуляторика, VPN, мессенджеры ———
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
	{Name: "Код Дурова", URL: "https://kod.ru/rss"},
	{Name: "Rozetked", URL: "https://rozetked.me/turbo"}, // RSS отдаётся только на /turbo
	{Name: "Kaspersky — блог", URL: "https://www.kaspersky.ru/blog/feed/"},

	// ——— Международные ———
	{Name: "Reddit r/VPN", URL: "https://www.reddit.com/r/VPN/.rss"},
}

// Article — нормализованная новость для фильтра и промпта.
type Article struct {
	Source      string
	Title       string
	Link        string
	Summary     string
	PublishedAt time.Time
	RUPriority  int // выше = приоритетнее для дайджеста про РФ
	Relevance   int // relevanceTopic / relevanceEntity
	Image       string
	EmojiID     string // свой анимированный эмодзи поста канала
	EmojiChar   string
	Ref         string   // статья, на которую ссылается пост канала
	Alt         *Article // у поста канала: статья сайта — запасной источник картинки
}

// topicKeywords — однозначная суть дайджеста: доступ, приватность,
// безопасность, регулирование связи. Одного такого слова достаточно.
var topicKeywords = []string{
	"vpn", "блокировк", "заблокир", "разблок", "обход", "запрещ",
	"приватность", "рунет", "цензур", "шифрован", "тспу", "dpi", "proxy",
	"взлом", "кибер", "утечк", "хакер", "слежк", "замедл",
	"роском", "ркн", "госдум", "минцифр", "законопроект", "регулятор",
	"биометри", "суверен", "персональн",
	"censorship", "privacy", "firewall", "surveillance", "leak", "breach",
}

// weakTopicKeywords сами по себе не значат ничего: «штраф», «мошенники», «суд»,
// «запрет» есть в любой криминальной или судебной сводке — так в дайджест
// попал оправдательный приговор основателю iSpring. Считаем темой только рядом
// с цифровым контекстом из digitalContextKeywords.
var weakTopicKeywords = []string{
	"штраф", "мошенн", "закон", "суд", "иск", "ограничен", "санкц", "запрет",
}

// excludeKeywords — чего в дайджесте быть не должно, даже когда рядом оказалось
// «судебное» и «цифровое» слово: так претензия Valve к скинченджеру для CS2
// прошла как «суд» + «сервис». Проверяются ПОСЛЕ topicKeywords, чтобы
// «Роскомнадзор заблокировал Steam» осталось новостью.
var excludeKeywords = []string{
	"игр", "геймер", "киберспорт", "читер", "скинченджер",
	"cs2", "cs:go", "dota", "steam", "valve",
	"товарный знак", "претензи", "досудебн", "обзор", "распродаж", "скидк",
}

var digitalContextKeywords = []string{
	"интернет", "онлайн", "сайт", "приложени", "мессенджер", "связ",
	"оператор", "сеть", "сети", "данн", "аккаунт", "сервис", "платформ",
	"трафик", "домен", "хостинг", "провайдер", "цифров", "соцсет",
	"telegram", "телеграм", "vpn", "youtube", "ютуб",
}

// entityKeywords — игроки рынка. Сами по себе НЕ делают новость нашей: «Яндекс
// выпустил колонку» или «МТС улучшила сеть» — это пресс-релиз, а не дайджест
// про приватность. Такие статьи берём только если тематических не хватило.
var entityKeywords = []string{
	"telegram", "телеграм", "whatsapp", "яндекс", "сбер", "вконтакте",
	"дуров", "ютуб", "youtube", "госуслуг", "мессенджер", "интернет",
	"apple", "google", "оператор связи",
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

const (
	feedTimeout  = 25 * time.Second // на одну попытку одной ленты
	feedMaxBytes = 5 << 20          // больше RSS-лента быть не должна
	feedWorkers  = 6
)

func fetchFeed(ctx context.Context, parser *gofeed.Parser, url string) (*gofeed.Feed, error) {
	ctx, cancel := context.WithTimeout(ctx, feedTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", parser.UserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return parser.Parse(io.LimitReader(resp.Body, feedMaxBytes))
}

func fetchFeedWithRetry(ctx context.Context, parser *gofeed.Parser, url string) (*gofeed.Feed, error) {
	var lastErr error
	for attempt := 1; attempt <= rssFetchAttempts; attempt++ {
		feed, err := fetchFeed(ctx, parser, url)
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

type feedResult struct {
	feed *gofeed.Feed
	err  error
}

// fetchAllFeeds качает все ленты параллельно; результаты в порядке feedSources.
func fetchAllFeeds(ctx context.Context, parser *gofeed.Parser) []feedResult {
	res := make([]feedResult, len(feedSources))
	sem := make(chan struct{}, feedWorkers)
	var wg sync.WaitGroup
	for i, src := range feedSources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res[i].feed, res[i].err = fetchFeedWithRetry(ctx, parser, googleNewsWhen7dURL(src.URL))
		}()
	}
	wg.Wait()
	return res
}

// FetchWeeklyArticles возвращает статьи и имена лент, которые не ответили.
func FetchWeeklyArticles(ctx context.Context, now time.Time) ([]Article, []string, error) {
	since := now.Add(-7 * 24 * time.Hour)
	parser := gofeed.NewParser()
	parser.Client = httpClient
	parser.UserAgent = "TreeShieldNewsBot/1.0 (+https://t.me/treeshield)"
	seen := make(map[string]struct{})
	var out []Article
	var failed []string

	results := fetchAllFeeds(ctx, parser)
	if ctx.Err() != nil {
		return nil, failed, ctx.Err()
	}
	for i, src := range feedSources {
		feed, err := results[i].feed, results[i].err
		if err != nil {
			log.Printf("RSS %q: пропуск (%v)", src.Name, err)
			failed = append(failed, src.Name)
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
			pub, ok := itemPublishedAt(item, now.Location())
			if !ok || pub.Before(since) {
				continue
			}
			if pub.After(now.Add(24 * time.Hour)) {
				continue
			}
			title := cleanText(item.Title)
			summary := cleanText(shortSummary(item))
			if textImpliesOlderThan(title, summary, since) {
				continue
			}
			relevance := articleRelevance(title, summary)
			if relevance == relevanceNone {
				continue
			}

			link := item.Link
			if link == "" {
				link = firstLink(item.Links)
			}
			key := dedupeKey(title, link)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			perFeed++

			out = append(out, Article{
				Source:      src.Name,
				Title:       title,
				Link:        link,
				Summary:     summary,
				PublishedAt: pub,
				RUPriority:  RuNewsPriority(title, summary),
				Relevance:   relevance,
				Image:       itemImage(item),
			})
		}
	}

	if len(out) == 0 {
		return nil, failed, fmt.Errorf("за последние 7 дней не найдено статей по ключевым словам")
	}
	sortArticlesForPrompt(out)
	log.Printf("RSS: ответили %d из %d лент", len(feedSources)-len(failed), len(feedSources))
	return out, failed, nil
}

func RuNewsPriority(title, summary string) int {
	text := strings.ToLower(title + " " + summary)
	score := 0
	for _, kw := range ruBoostKeywords {
		if strings.Contains(text, kw) {
			score++
		}
	}
	return score
}

// sortArticlesForPrompt — сначала по теме, затем про РФ, внутри группы по дате.
// Порядок важен: в промпт уходят первые maxArticlesInPrompt статей, поэтому
// пресс-релизы должны оставаться в хвосте и вытесняться, а не наоборот.
func sortArticlesForPrompt(articles []Article) {
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].Relevance != articles[j].Relevance {
			return articles[i].Relevance > articles[j].Relevance
		}
		if articles[i].RUPriority != articles[j].RUPriority {
			return articles[i].RUPriority > articles[j].RUPriority
		}
		return articles[i].PublishedAt.After(articles[j].PublishedAt)
	})
}

// Уровни релевантности статьи для дайджеста.
const (
	relevanceNone   = 0 // мимо темы
	relevanceEntity = 1 // упомянут игрок, но сути нет — запасной эшелон
	relevanceTopic  = 2 // блокировки, приватность, ИБ, регулирование
)

// articleRelevance различает новость по теме и корпоративный пресс-релиз, где
// просто мелькает знакомый бренд. Раньше фильтр был плоским ИЛИ по общей куче
// слов, и «МТС улучшила мобильный интернет» проходило наравне с блокировками.
func articleRelevance(title, summary string) int {
	text := strings.ToLower(title + " " + summary)
	if containsAny(text, topicKeywords) {
		return relevanceTopic
	}
	if containsAny(text, excludeKeywords) {
		return relevanceNone
	}
	if containsAny(text, weakTopicKeywords) && containsAny(text, digitalContextKeywords) {
		return relevanceTopic
	}
	if containsAny(text, entityKeywords) {
		return relevanceEntity
	}
	return relevanceNone
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if matchesKeyword(text, kw) {
			return true
		}
	}
	return false
}

// matchesKeyword ищет ключевое слово с начала слова. Подстрока без этой
// проверки давала мусор: «иск» находился внутри «поиска», «рисков» и «диска»,
// так что статья про риски рынка попадала в дайджест про приватность.
func matchesKeyword(text, kw string) bool {
	for from := 0; from <= len(text)-len(kw); {
		i := strings.Index(text[from:], kw)
		if i < 0 {
			return false
		}
		abs := from + i
		if abs == 0 {
			return true
		}
		r, _ := utf8.DecodeLastRuneInString(text[:abs])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
		from = abs + len(kw)
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
		return truncate(StripHTML(item.Description), maxSummaryRunes)
	}
	if item.Content != "" {
		return truncate(StripHTML(item.Content), maxSummaryRunes)
	}
	return ""
}

func StripHTML(s string) string {
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
	return strings.TrimSpace(StripHTML(s))
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func ArticlesForPrompt(all []Article) []Article {
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
	if r := []rune(name); len(r) > 18 {
		return string(r[:18])
	}
	return name
}

func BuildDigestPrompt(articles []Article, maxItems int) string {
	pool := ArticlesForPrompt(articles)
	if maxItems > 0 && len(pool) > maxItems {
		pool = pool[:maxItems]
	}
	var b strings.Builder
	b.WriteString("Лента за последние 7 дней (приоритет — Россия/рунет; сначала идут основные источники «Код Дурова» и «Rozetked»: если подходящая тема есть у них — бери оттуда, остальные ленты только как резерв, когда их не хватает). Бери только строки с датой в окне:\n")
	for i, a := range pool {
		line := fmt.Sprintf("%d.%s|%s|%s|%s",
			i+1,
			a.PublishedAt.Format("02.01.2006"),
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

// FeedNames — имена всех лент (для подсчёта доли упавших).
func FeedNames() []string {
	names := make([]string, len(feedSources))
	for i, s := range feedSources {
		names[i] = s.Name
	}
	return names
}

// itemImage — обложка из enclosure (у kod.ru тип не указан); SVG пропускаем.
func itemImage(item *gofeed.Item) string {
	for _, e := range item.Enclosures {
		if e.URL != "" && (e.Type == "" || strings.HasPrefix(e.Type, "image/")) && !isSVG(e.URL) {
			return e.URL
		}
	}
	return "" // item.Image не берём: у kod.ru там фавиконка сайта; дальше сработает og:image
}

func isSVG(u string) bool {
	return strings.HasSuffix(strings.ToLower(strings.SplitN(u, "?", 2)[0]), ".svg")
}

var ogImageRE = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']|<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)

var tgPhotoRE = regexp.MustCompile(`tgme_widget_message_photo_wrap[^>]*background-image:url\('([^']+)'\)`)

// ImageFor: картинка статьи; у поста канала сначала его фото, и только если нет — картинка статьи сайта (Alt).
func ImageFor(ctx context.Context, a Article) string {
	if u := imageOf(ctx, a); u != "" || a.Alt == nil {
		return u
	}
	return imageOf(ctx, *a.Alt)
}

// imageOf возвращает картинку статьи: из RSS, иначе og:image страницы. Пусто при любой ошибке.
func imageOf(ctx context.Context, a Article) string {
	if a.Image != "" {
		return a.Image
	}
	if a.Link == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// у t.me-поста без фото og:image — аватар канала; фото есть только в блоке photo_wrap embed-версии
	link := strings.Replace(a.Link, "t.me/s/", "t.me/", 1)
	isTG := strings.Contains(link, "://t.me/")
	if isTG {
		link += "?embed=1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	var u string
	if isTG {
		m := tgPhotoRE.FindSubmatch(b)
		if m == nil {
			return ""
		}
		u = html.UnescapeString(string(m[1]))
	} else {
		m := ogImageRE.FindSubmatch(b)
		if m == nil {
			return ""
		}
		u = html.UnescapeString(string(m[1]) + string(m[2]))
	}
	if !strings.HasPrefix(u, "http") || isSVG(u) {
		return ""
	}
	return u
}

// DownloadImage скачивает картинку (≤10 МБ); nil при любой ошибке или не-картинке.
func DownloadImage(ctx context.Context, u string) []byte {
	if !strings.HasPrefix(u, "http") {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// OnTopic — статья по сути темы дайджеста (не пресс-релиз и не просто упомянутый бренд).
func OnTopic(a Article) bool { return a.Relevance >= relevanceTopic }

// FilterOnTopic оставляет тематические статьи; если их меньше min, отдаёт всё как есть
// (пунктов всегда шесть, лучше слабая тема, чем сорванный выпуск).
func FilterOnTopic(all []Article, min int) []Article {
	out := make([]Article, 0, len(all))
	for _, a := range all {
		if OnTopic(a) {
			out = append(out, a)
		}
	}
	if len(out) < min {
		return all
	}
	return out
}
