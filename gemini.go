package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	geminiMaxBatchAttempts   = 4
	geminiMaxAPIRetries      = 5
	geminiMaxOutputTokens    = 4800
	geminiSingleOutputTokens = 1200
	geminiSequentialArticles = 18
)

var singleNewsNumRE = regexp.MustCompile(`<b>\s*\d{1,2}\.\s`)

func generateDigest(ctx context.Context, cfg Config, articles []Article) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}

	fullPrompt := buildNewsDigestPrompt(articles)
	body, err := generateDigestBatch(ctx, client, cfg, fullPrompt)
	if err == nil {
		return body, nil
	}
	log.Printf("Пакетная генерация не удалась (%v), пробуем по одной новости…", err)

	compactPrompt := buildCompactDigestPrompt(articles, geminiSequentialArticles)
	return generateDigestSequential(ctx, client, cfg, compactPrompt)
}

func generateDigestBatch(ctx context.Context, client *genai.Client, cfg Config, userText string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= geminiMaxBatchAttempts; attempt++ {
		prompt := userText
		if attempt > 1 {
			prompt += "\n\nПредыдущий ответ не подошёл. Верни все 6 пунктов (1–5 Россия, 6 — зарубеж). Заголовки до 10 слов. В каждом — 2 коротких предложения."
		}

		body, reason, err := callGemini(ctx, client, cfg, systemPrompt, prompt, attempt, geminiMaxOutputTokens)
		if err != nil {
			lastErr = err
			if isGeminiRetryable(err) {
				log.Printf("Gemini пакет, попытка %d: %v", attempt, err)
				continue
			}
			return "", err
		}

		body = sanitizeNewsBody(body)
		if err := validateNewsBody(body); err != nil {
			lastErr = fmt.Errorf("%v (finish=%s)", err, reason)
			log.Printf("Gemini пакет, попытка %d: %v", attempt, lastErr)
			continue
		}
		if attempt > 1 {
			log.Printf("Gemini пакет: успешно с попытки %d", attempt)
		}
		return body, nil
	}
	return "", lastErr
}

func generateDigestSequential(ctx context.Context, client *genai.Client, cfg Config, feed string) (string, error) {
	var parts []string
	var usedTitles []string

	for n := 1; n <= requiredNewsItems; n++ {
		body, err := generateSingleNewsItem(ctx, client, cfg, feed, n, usedTitles)
		if err != nil {
			return "", err
		}
		parts = append(parts, body)
		if title := extractNewsTitle(body); title != "" {
			usedTitles = append(usedTitles, title)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}

func generateSingleNewsItem(
	ctx context.Context,
	client *genai.Client,
	cfg Config,
	feed string,
	number int,
	used []string,
) (string, error) {
	prompt := buildSingleNewsPrompt(feed, number, used)
	tokens := []int32{geminiSingleOutputTokens, 1800, 2400}
	var lastErr error

	for i, maxOut := range tokens {
		extra := ""
		if i > 0 {
			extra = "\n\nОтветь только одним пунктом: <b>N. Заголовок</b> и 2 коротких предложения."
		}
		body, reason, err := callGemini(ctx, client, cfg, systemPromptSingle, prompt+extra, i+1, maxOut)
		if err != nil {
			if isGeminiRetryable(err) {
				lastErr = err
				continue
			}
			return "", err
		}
		body = sanitizeNewsBody(normalizeSingleNewsBlock(body, number))
		if err := validateSingleNewsBlock(body); err != nil {
			lastErr = fmt.Errorf("%w (finish=%s)", err, reason)
			log.Printf("Gemini пункт %d (попытка %d): %v", number, i+1, lastErr)
			continue
		}
		return body, nil
	}
	return "", fmt.Errorf("пункт %d: %w", number, lastErr)
}

const systemPromptSingle = `Редактор IT-дайджеста. Выбери ОДНУ новость по запросу.
<b>N. Заголовок</b> — до 10 слов, до 80 символов.
2 коротких предложения. Без emoji, вступлений, рекламы, markdown, URL.`

func buildSingleNewsPrompt(feed string, number int, used []string) string {
	var b strings.Builder
	b.WriteString("Лента за 7 дней:\n")
	b.WriteString(feed)
	b.WriteByte('\n')
	if number == foreignNewsItemNum {
		b.WriteString("\nНужен пункт <b>6.</b> — одна важная ЗАРУБЕЖНАЯ новость (не про Россию).\n")
	} else {
		fmt.Fprintf(&b, "\nНужен пункт <b>%d.</b> — новость про Россию (РКН, Госдума, VPN, Telegram, Яндекс, рунет).\n", number)
	}
	if len(used) > 0 {
		b.WriteString("Уже выбраны темы (не повторяй): ")
		b.WriteString(strings.Join(used, "; "))
		b.WriteByte('\n')
	}
	return b.String()
}

func buildCompactDigestPrompt(articles []Article, limit int) string {
	pool := articlesForPrompt(articles)
	if len(pool) > limit {
		pool = pool[:limit]
	}
	var b strings.Builder
	b.WriteString("Лента 7д (приоритет — Россия/рунет):\n")
	for i, a := range pool {
		fmt.Fprintf(&b, "%d.%s|%s|%s|%s\n",
			i+1,
			a.PublishedAt.Format("02.01"),
			shortSource(a.Source),
			a.Title,
			a.Link,
		)
	}
	return b.String()
}

func normalizeSingleNewsBlock(body string, number int) string {
	body = strings.TrimSpace(body)
	if countNewsItems(body) == 0 {
		if !strings.HasPrefix(body, "<b>") {
			body = fmt.Sprintf("<b>%d. %s</b>\n%s", number, body, "")
		}
	}
	return singleNewsNumRE.ReplaceAllString(body, fmt.Sprintf("<b>%d. ", number))
}

func isGeminiRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "503") ||
		strings.Contains(s, "429") ||
		strings.Contains(s, "unavailable") ||
		strings.Contains(s, "high demand") ||
		strings.Contains(s, "resource exhausted") ||
		strings.Contains(s, "deadline exceeded")
}

func callGemini(
	ctx context.Context,
	client *genai.Client,
	cfg Config,
	systemPrompt, userText string,
	attempt int,
	maxOut int32,
) (string, genai.FinishReason, error) {
	temp := float32(0.45)
	if attempt > 1 {
		temp = 0.3
	}

	contents := []*genai.Content{
		{Parts: []*genai.Part{{Text: userText}}},
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		Temperature:     &temp,
		MaxOutputTokens: maxOut,
	}

	var lastErr error
	for try := 1; try <= geminiMaxAPIRetries; try++ {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}

		result, err := client.Models.GenerateContent(ctx, cfg.GeminiModel, contents, config)
		if err != nil {
			lastErr = fmt.Errorf("generate content: %w", err)
			if isGeminiRetryable(err) && try < geminiMaxAPIRetries {
				wait := time.Duration(try*try) * time.Second
				log.Printf("Gemini API: %v — повтор через %s (%d/%d)", err, wait, try, geminiMaxAPIRetries)
				select {
				case <-ctx.Done():
					return "", "", ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
			return "", "", lastErr
		}

		text := strings.TrimSpace(result.Text())
		reason := genai.FinishReasonUnspecified
		if len(result.Candidates) > 0 {
			reason = result.Candidates[0].FinishReason
		}
		if text == "" {
			lastErr = fmt.Errorf("пустой ответ от Gemini (finish=%s)", reason)
			if try < geminiMaxAPIRetries {
				time.Sleep(time.Duration(try) * time.Second)
				continue
			}
			return "", reason, lastErr
		}
		if reason == genai.FinishReasonMaxTokens {
			log.Printf("Gemini: ответ обрезан (MAX_TOKENS, лимит %d)", maxOut)
		}
		return text, reason, nil
	}
	return "", "", lastErr
}
