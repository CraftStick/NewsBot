package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	titleEmojiID      = "5764829255015861596"
	titleEmojiText    = "🗣"
	subtitleEmojiID   = "5951891646445523477"
	subtitleEmojiFB   = "📰"
	closingEmojiID    = "5798587088077066898"
	closingEmojiFB    = "👋"
	newsBulletEmojiID = "5429501538806548545"
	newsBulletEmojiFB = "✅"
	requiredNewsItems  = 6
	russianNewsItems   = 5 // пункты 1–5: Россия
	foreignNewsItemNum = 6 // пункт 6: зарубежная новость
	minNewsTextRunes  = 40
	maxNewsTextRunes  = 320
	maxNewsTitleRunes = 85
)

var sanitizePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*<b>\s*«?Пятничный дайджест»?\s*</b>.*\n?`),
	regexp.MustCompile(`(?i).*[Пп]ятничный дайджест.*\n?`),
	regexp.MustCompile(`(?i).*[Гг]лавные события в мире приватности.*\n?`),
	regexp.MustCompile(`(?i)\s*<blockquote>[\s\S]*?</blockquote>\s*`),
	regexp.MustCompile(`(?i)\s*<i>\s*Увидимся в следующую пятницу[^<]*</i>\s*`),
	regexp.MustCompile(`(?i)\s*Увидимся в следующую пятницу[^\n]*\n?`),
	regexp.MustCompile(`<tg-emoji[^>]*>[\s\S]*?</tg-emoji>\s*`),
	regexp.MustCompile("(?m)^```[a-z]*\\n?|```$"),
	regexp.MustCompile(`(?m)^\s*[🔴✅•\-]\s*`),
	regexp.MustCompile(`(?im)^.*(самые заметные|главные события|итоги недели|вот что).*\n`),
}

var (
	newsItemHeading = regexp.MustCompile(`<b>\s*(\d{1,2})\.\s`)
	firstNewsItem   = regexp.MustCompile(`(?is)<b>\s*1\.\s`)
)

func tgEmoji(id, fallback string) string {
	return fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, fallback)
}

func countNewsItems(body string) int {
	return len(newsItemHeading.FindAllStringIndex(body, -1))
}

func stripPreamble(body string) string {
	loc := firstNewsItem.FindStringIndex(body)
	if loc != nil && loc[0] > 0 {
		return strings.TrimSpace(body[loc[0]:])
	}
	return strings.TrimSpace(body)
}

func splitNewsBlocks(body string) []string {
	locs := newsItemHeading.FindAllStringIndex(body, -1)
	if len(locs) == 0 {
		return nil
	}
	blocks := make([]string, 0, len(locs))
	for i, loc := range locs {
		start := loc[0]
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		blocks = append(blocks, strings.TrimSpace(body[start:end]))
	}
	return blocks
}

var headlineSourceSuffix = regexp.MustCompile(`\s*[-—–|]\s*[\p{L}\p{N}«»"'. ]{1,45}$`)

// trimHeadline укорачивает заголовок для Telegram (ссылки подбираются по полному тексту).
func trimHeadline(s string) string {
	s = strings.TrimSpace(stripHTML(s))
	s = headlineSourceSuffix.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxNewsTitleRunes {
		return s
	}
	cut := string(runes[:maxNewsTitleRunes])
	if sp := strings.LastIndex(cut, " "); sp > len(cut)/2 {
		cut = cut[:sp]
	}
	return strings.TrimSpace(cut) + "…"
}

func newsBlockBody(block string) string {
	if i := strings.LastIndex(block, "</b>"); i >= 0 {
		return strings.TrimSpace(block[i+len("</b>"):])
	}
	if nl := strings.Index(block, "\n"); nl >= 0 {
		return strings.TrimSpace(block[nl+1:])
	}
	return ""
}

func textEndsComplete(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	runes := []rune(text)
	last := runes[len(runes)-1]
	return unicode.IsPunct(last) || last == '»' || last == '…'
}

func validateSingleNewsBlock(body string) error {
	blocks := splitNewsBlocks(strings.TrimSpace(body))
	if len(blocks) == 0 {
		return fmt.Errorf("нет блока новости")
	}
	text := newsBlockBody(blocks[0])
	runes := []rune(text)
	if len(runes) < minNewsTextRunes {
		return fmt.Errorf("слишком короткий (%d симв.)", len(runes))
	}
	if len(runes) > maxNewsTextRunes {
		return fmt.Errorf("слишком длинный (%d симв.)", len(runes))
	}
	if !textEndsComplete(text) {
		return fmt.Errorf("текст оборван")
	}
	return nil
}

// validateNewsBody проверяет: 5 пунктов, каждый с полным текстом.
func validateNewsBody(body string) error {
	body = strings.TrimSpace(body)
	n := countNewsItems(body)
	if n < requiredNewsItems {
		return fmt.Errorf("пунктов %d из %d", n, requiredNewsItems)
	}
	blocks := splitNewsBlocks(body)
	if len(blocks) < requiredNewsItems {
		return fmt.Errorf("не удалось разобрать блоки новостей")
	}
	for i, block := range blocks[:requiredNewsItems] {
		title := extractNewsTitle(block)
		if len([]rune(title)) > maxNewsTitleRunes {
			return fmt.Errorf("пункт %d: заголовок слишком длинный (%d симв.)", i+1, len([]rune(title)))
		}
		text := newsBlockBody(block)
		runes := []rune(text)
		if len(runes) < minNewsTextRunes {
			return fmt.Errorf("пункт %d слишком короткий (%d симв.)", i+1, len(runes))
		}
		if len(runes) > maxNewsTextRunes {
			return fmt.Errorf("пункт %d слишком длинный (%d симв.)", i+1, len(runes))
		}
		if !textEndsComplete(text) {
			return fmt.Errorf("пункт %d обрывается на полуслове", i+1)
		}
	}
	return nil
}

func sanitizeNewsBody(body string) string {
	body = strings.TrimSpace(body)
	for _, re := range sanitizePatterns {
		body = strings.TrimSpace(re.ReplaceAllString(body, ""))
	}
	return stripPreamble(body)
}

func injectNewsBullets(body string) string {
	bullet := tgEmoji(newsBulletEmojiID, newsBulletEmojiFB)
	return newsItemHeading.ReplaceAllString(body, bullet+` <b>$1. `)
}

func assembleDigest(newsBody string) string {
	newsBody = injectNewsBullets(sanitizeNewsBody(newsBody))

	var b strings.Builder
	b.WriteString(`<b>«Пятничный дайджест»</b> `)
	b.WriteString(tgEmoji(titleEmojiID, titleEmojiText))
	b.WriteString("\n\n")
	b.WriteString(tgEmoji(subtitleEmojiID, subtitleEmojiFB))
	b.WriteString(" Главные события в мире приватности и технологий за неделю:\n\n")
	b.WriteString(newsBody)
	b.WriteString("\n\n<blockquote><i>Увидимся в следующую пятницу, удачи!</i> ")
	b.WriteString(tgEmoji(closingEmojiID, closingEmojiFB))
	b.WriteString("</blockquote>")
	return b.String()
}
