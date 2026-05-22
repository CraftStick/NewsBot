package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

var (
	staleMonthYearRE = regexp.MustCompile(`(?i)(январ|феврал|март|апрел|мая|июн|июл|август|сентябр|октябр|ноябр|декабр)[а-яё]*\s+(\d{4})`)
	explicitYearRE   = regexp.MustCompile(`(?:^|[^\d])(20\d{2})(?:[^\d]|$)`)
)

func monthFromStem(stem string) time.Month {
	stem = strings.ToLower(stem)
	switch {
	case strings.HasPrefix(stem, "январ"):
		return time.January
	case strings.HasPrefix(stem, "феврал"):
		return time.February
	case strings.HasPrefix(stem, "март"):
		return time.March
	case strings.HasPrefix(stem, "апрел"):
		return time.April
	case strings.HasPrefix(stem, "мая"):
		return time.May
	case strings.HasPrefix(stem, "июн"):
		return time.June
	case strings.HasPrefix(stem, "июл"):
		return time.July
	case strings.HasPrefix(stem, "август"):
		return time.August
	case strings.HasPrefix(stem, "сентябр"):
		return time.September
	case strings.HasPrefix(stem, "октябр"):
		return time.October
	case strings.HasPrefix(stem, "ноябр"):
		return time.November
	case strings.HasPrefix(stem, "декабр"):
		return time.December
	default:
		return 0
	}
}

// itemPublishedAt — только дата публикации (не Updated: иначе старые статьи проходят как «свежие»).
func itemPublishedAt(item *gofeed.Item, loc *time.Location) (time.Time, bool) {
	if item == nil {
		return time.Time{}, false
	}
	if item.PublishedParsed != nil {
		return item.PublishedParsed.In(loc), true
	}
	if item.Published != "" {
		if t, err := time.ParseInLocation(time.RFC1123Z, item.Published, loc); err == nil {
			return t, true
		}
		if t, err := time.ParseInLocation(time.RFC1123, item.Published, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// textImpliesOlderThan — в тексте явно указан месяц+год до начала окна.
func textImpliesOlderThan(title, summary string, since time.Time) bool {
	text := strings.ToLower(title + " " + summary)

	for _, m := range staleMonthYearRE.FindAllStringSubmatch(text, -1) {
		if len(m) < 3 {
			continue
		}
		month := monthFromStem(m[1])
		if month == 0 {
			continue
		}
		y, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		end := time.Date(y, month+1, 0, 23, 59, 0, 0, since.Location())
		if end.Before(since) {
			return true
		}
	}

	for _, m := range explicitYearRE.FindAllStringSubmatch(text, -1) {
		y, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if y < since.Year() {
			return true
		}
	}
	return false
}

func googleNewsWhen7dURL(url string) string {
	if !strings.Contains(url, "news.google.com/rss/search") {
		return url
	}
	if strings.Contains(url, "when:") {
		return url
	}
	if strings.Contains(url, "&hl=") {
		return strings.Replace(url, "&hl=", "+when:7d&hl=", 1)
	}
	return url + "+when:7d"
}
