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
	foreignNewsItemNum = 6 // пункт 6: зарубежная новость
	telegramMaxMessage = 4096
	telegramMaxCaption = 1000 // запас к лимиту подписи Telegram (1024, считается после парсинга сущностей)
	minNewsTextRunes  = 40
	maxNewsTextRunes  = 320
	maxNewsTitleRunes = 85
)

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// captionVisibleLen — длина видимого текста (Telegram считает подпись без разметки:
// теги <b>/<a>/<tg-emoji> и URL в href не в счёт, остаётся сам текст и эмодзи).
func captionVisibleLen(html string) int {
	return len([]rune(strings.TrimSpace(htmlTagRE.ReplaceAllString(html, ""))))
}

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

// Отсекаем хвост-источник Google News («Заголовок - РИА Новости»): дефис/пайп
// с пробелами с обеих сторон.
var headlineSourceSuffix = regexp.MustCompile(`\s+[-|]\s+[\p{L}\p{N}«»"'. ]{1,45}$`)

// Через длинное тире отсекаем ТОЛЬКО явный источник: в «кавычках» или 1–2 слова
// с заглавной («— Ведомости», «— РИА Новости»). Обычную пунктуацию
// («VPN в России — что изменится») не трогаем — тире в русском это знак препинания.
var headlineEmDashSource = regexp.MustCompile(`\s+[—–]\s+(?:«[^»]{1,40}»|(?:[A-ZА-ЯЁ][\p{L}.]*\s*){1,2})$`)

// trimHeadline укорачивает заголовок для Telegram (ссылки подбираются по полному тексту).
func trimHeadline(s string) string {
	s = strings.TrimSpace(stripHTML(s))
	s = headlineSourceSuffix.ReplaceAllString(s, "")
	s = headlineEmDashSource.ReplaceAllString(s, "")
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

// validateNewsBody проверяет 6 пунктов: заголовок и два полных предложения в каждом.
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

// assembleDigestForCaption собирает дайджест для подписи к фото и, если он не
// влезает в лимит Telegram, аккуратно ужимает тексты пунктов. ok=false — не
// удалось ужать (тогда вызывающий шлёт фото и текст раздельно).
func assembleDigestForCaption(newsBody string) (string, bool) {
	body, ok := fitNewsBodyToCaption(newsBody, telegramMaxCaption)
	if !ok {
		return "", false
	}
	return assembleDigest(body), true
}

func fitNewsBodyToCaption(newsBody string, maxVisible int) (string, bool) {
	if captionVisibleLen(assembleDigest(newsBody)) <= maxVisible {
		return newsBody, true
	}
	// Шаг 1: оставить по одному предложению в каждом пункте.
	one := mapItemBodies(newsBody, firstSentence)
	if captionVisibleLen(assembleDigest(one)) <= maxVisible {
		return one, true
	}
	// Шаг 2: подрезать тела пунктов до убывающего бюджета символов.
	for _, budget := range []int{130, 100, 80, 60, 45} {
		trimmed := mapItemBodies(one, func(s string) string { return trimToRunes(s, budget) })
		if captionVisibleLen(assembleDigest(trimmed)) <= maxVisible {
			return trimmed, true
		}
	}
	return "", false
}

// mapItemBodies применяет fn к тексту каждого пункта, сохраняя строку-заголовок
// (<b>N. <a…>…</a></b>) без изменений.
func mapItemBodies(newsBody string, fn func(string) string) string {
	blocks := splitNewsBlocks(newsBody)
	if len(blocks) == 0 {
		return newsBody
	}
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		idx := strings.LastIndex(b, "</b>")
		if idx < 0 {
			out = append(out, b)
			continue
		}
		head := b[:idx+len("</b>")]
		body := strings.TrimSpace(b[idx+len("</b>"):])
		out = append(out, head+"\n"+strings.TrimSpace(fn(body)))
	}
	return strings.Join(out, "\n\n")
}

// firstSentence возвращает первое предложение (до . ! ? … перед пробелом/концом).
func firstSentence(s string) string {
	r := []rune(strings.TrimSpace(s))
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '.', '!', '?', '…':
			if i+1 >= len(r) || r[i+1] == ' ' {
				return strings.TrimSpace(string(r[:i+1]))
			}
		}
	}
	return strings.TrimSpace(string(r))
}

func trimToRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	cut := string(r[:max])
	if sp := strings.LastIndex(cut, " "); sp > max/2 {
		cut = cut[:sp]
	}
	return strings.TrimSpace(cut) + "…"
}
