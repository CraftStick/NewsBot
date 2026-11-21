package news

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/mmcdole/gofeed"
	"time"

	"treesheild-newsbot/internal/settings"
)

// miniFeeds — источники мини-постов.
var miniFeeds = []struct {
	Name, URL string
	All       bool // без фильтра по теме
}{
	{"Код Дурова", "https://kod.ru/rss", false},
}

// OnSourceDown вызывается, когда источник не отвечает miniDownAfter проходов подряд.
var OnSourceDown func(name string)

const miniDownAfter = 15

var (
	miniFailsMu sync.Mutex
	miniFails   = map[string]int{}
	miniLastOK  = map[string]time.Time{}
)

func feedID(name string) string   { return "RSS " + name }
func chanID(handle string) string { return "Telegram @" + handle }

// SourceIDs — идентификаторы источников для настроек.
func SourceIDs() []string {
	var ids []string
	for _, f := range miniFeeds {
		ids = append(ids, feedID(f.Name))
	}
	for _, c := range miniChannels {
		ids = append(ids, chanID(c.Handle))
	}
	return ids
}

// SourceState — состояние источника для страницы статуса.
type SourceState struct {
	ID     string    `json:"id"`
	Fails  int       `json:"fails"`
	LastOK time.Time `json:"last_ok"`
}

func SourceStatus() []SourceState {
	miniFailsMu.Lock()
	defer miniFailsMu.Unlock()
	loadOK()
	out := []SourceState{}
	for _, id := range SourceIDs() {
		out = append(out, SourceState{id, miniFails[id], miniLastOK[id]})
	}
	return out
}

// noteSource считает неудачи подряд; сообщение — один раз на сбой.
func noteSource(name string, err error) {
	miniFailsMu.Lock()
	loadOK()
	if err == nil {
		miniFails[name] = 0
		miniLastOK[name] = time.Now()
		saveOK()
		miniFailsMu.Unlock()
		return
	}
	miniFails[name]++
	down := miniFails[name] == miniDownAfter
	miniFailsMu.Unlock()
	if down && OnSourceDown != nil {
		OnSourceDown(name)
	}
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
		if settings.IsOff(feedID(src.Name)) {
			continue
		}
		if !sourceDue(feedID(src.Name), time.Now()) {
			continue
		}
		feed, err := fetchFeedCondRetry(ctx, parser, src.URL)
		noteSource(feedID(src.Name), err)
		if err != nil {
			log.Printf("мини-посты: %s: %v", src.Name, err)
			errs = append(errs, err)
			continue
		}
		if feed == nil { // 304: лента не менялась
			continue
		}
		for _, it := range feed.Items {
			link := itemLink(it)
			if link == "" || strings.TrimSpace(it.Title) == "" || it.PublishedParsed == nil {
				continue
			}
			title, sum := cleanMiniText(it.Title, miniSummary(it))
			// Только по теме канала (блокировки, приватность, ИБ), как в дайджесте.
			rel := articleRelevance(title, sum)
			if !src.All && (rel < relevanceTopic || !miniCore(title, sum)) || isRoundup(title) {
				continue
			}
			out = append(out, Article{Source: src.Name, Title: title, Link: link,
				Summary: sum, PublishedAt: *it.PublishedParsed, Relevance: rel})
		}
	}
	for _, ch := range miniChannels {
		if settings.IsOff(chanID(ch.Handle)) {
			continue
		}
		if !sourceDue(chanID(ch.Handle), time.Now()) {
			continue
		}
		arts, err := fetchChannel(ctx, ch.Name, ch.Handle)
		noteSource(chanID(ch.Handle), err)
		if err != nil {
			log.Printf("мини-посты: %s: %v", ch.Handle, err)
			errs = append(errs, err)
			continue
		}
		for _, a := range arts {
			if a.Relevance = articleRelevance(a.Title, a.Summary); a.Relevance >= relevanceTopic && miniCore(a.Title, a.Summary) && !isRoundup(a.Title) {
				out = append(out, a)
			}
		}
	}
	if len(errs) == len(miniFeeds)+len(miniChannels) {
		return nil, errors.Join(errs...)
	}
	out = dropRefDupes(out)
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

// isRoundup — рубрика-подборка ссылок («Компании на kod.ru»), а не новость.
func isRoundup(title string) bool {
	return strings.HasPrefix(fold(title), fold("Компании на kod.ru"))
}

// miniCore: тема по общему фильтру, но «блокировка экрана» — не та блокировка
// (так прошла новость про уязвимость Windows).
func miniCore(title, summary string) bool {
	r := strings.NewReplacer("экрана блокировки", "", "экран блокировки", "", "блокировка экрана", "", "блокировки экрана", "")
	return articleRelevance(r.Replace(strings.ToLower(title)), r.Replace(strings.ToLower(summary))) >= relevanceTopic
}

// itemLink — ссылка записи; нет <link> — берём guid, если это адрес (запасной ключ).
func itemLink(it *gofeed.Item) string {
	if it.Link != "" {
		return it.Link
	}
	if strings.HasPrefix(it.GUID, "http://") || strings.HasPrefix(it.GUID, "https://") {
		return it.GUID
	}
	return ""
}
