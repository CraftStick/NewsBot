package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Фиксированная шапка и подвал дайджеста (не генерируются нейросетью).
const (
	titleEmojiID    = "5764829255015861596"
	titleEmojiText  = "🗣"
	subtitleEmojiID = "5951891646445523477"
	subtitleEmojiFB = "📰"
	closingEmojiID  = "5798587088077066898"
	closingEmojiFB  = "👋"
	newsBulletEmojiID = "5429501538806548545"
	newsBulletEmojiFB = "✅"
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
}

// Заголовки новостей от Gemini: <b>1. Название</b>
var newsItemHeading = regexp.MustCompile(`<b>\s*(\d{1,2})\.\s`)

func tgEmoji(id, fallback string) string {
	return fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, fallback)
}

// sanitizeNewsBody убирает из ответа Gemini случайные заголовки и прощания.
func sanitizeNewsBody(body string) string {
	body = strings.TrimSpace(body)
	for _, re := range sanitizePatterns {
		body = strings.TrimSpace(re.ReplaceAllString(body, ""))
	}
	return strings.TrimSpace(body)
}

// injectNewsBullets добавляет анимированный ✅ перед каждым пунктом <b>1. …</b>.
func injectNewsBullets(body string) string {
	bullet := tgEmoji(newsBulletEmojiID, newsBulletEmojiFB)
	return newsItemHeading.ReplaceAllString(body, bullet+` <b>$1. `)
}

// assembleDigest склеивает шаблон канала и сгенерированные новости.
func assembleDigest(newsBody string) string {
	newsBody = injectNewsBullets(sanitizeNewsBody(newsBody))

	var b strings.Builder
	b.WriteString(`<b>«Пятничный дайджест»</b> `)
	b.WriteString(tgEmoji(titleEmojiID, titleEmojiText))
	b.WriteString("\n\n")
	b.WriteString(tgEmoji(subtitleEmojiID, subtitleEmojiFB))
	b.WriteString(" Главные события в мире приватности и технологий за неделю:\n\n")
	b.WriteString(newsBody)
	b.WriteString("\n\n<blockquote><i>Увидимся в следующую пятницу, удачи!</i></blockquote> ")
	b.WriteString(tgEmoji(closingEmojiID, closingEmojiFB))
	return b.String()
}
