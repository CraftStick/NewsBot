package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

const (
	geminiMaxBatchAttempts = 3
	geminiMaxOutputTokens  = 4800
)

func generateDigest(ctx context.Context, cfg Config, rawNews string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}

	body, err := generateDigestBatch(ctx, client, cfg, rawNews)
	if err == nil {
		return body, nil
	}
	log.Printf("Пакетная генерация не удалась (%v), пробуем по одной новости…", err)

	return generateDigestSequential(ctx, client, cfg, rawNews)
}

func generateDigestBatch(ctx context.Context, client *genai.Client, cfg Config, rawNews string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= geminiMaxBatchAttempts; attempt++ {
		userText := rawNews
		if attempt > 1 {
			userText += "\n\nПредыдущий ответ не подошёл. Верни все 6 пунктов (1–5 Россия, 6 — зарубеж). Заголовки до 10 слов. В каждом — 2 коротких предложения."
		}

		body, reason, err := callGemini(ctx, client, cfg, systemPrompt, userText, attempt, geminiMaxOutputTokens)
		if err != nil {
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

func generateDigestSequential(ctx context.Context, client *genai.Client, cfg Config, rawNews string) (string, error) {
	var parts []string
	var usedTitles []string

	for n := 1; n <= requiredNewsItems; n++ {
		prompt := buildSingleNewsPrompt(rawNews, n, usedTitles)
		body, reason, err := callGemini(ctx, client, cfg, systemPromptSingle, prompt, 1, 512)
		if err != nil {
			return "", err
		}
		body = sanitizeNewsBody(body)
		body = normalizeSingleNewsBlock(body, n)

		if err := validateSingleNewsBlock(body); err != nil {
			log.Printf("Gemini пункт %d: %v (finish=%s), повтор…", n, err, reason)
			body, reason, err = callGemini(ctx, client, cfg, systemPromptSingle, prompt+"\n\nСократи до 2 коротких предложений.", 2, 768)
			if err != nil {
				return "", err
			}
			body = sanitizeNewsBody(normalizeSingleNewsBlock(body, n))
			if err := validateSingleNewsBlock(body); err != nil {
				return "", fmt.Errorf("пункт %d: %w (finish=%s)", n, err, reason)
			}
		}

		parts = append(parts, body)
		if title := extractNewsTitle(body); title != "" {
			usedTitles = append(usedTitles, title)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}

const systemPromptSingle = `Редактор IT-дайджеста. Выбери ОДНУ новость по запросу.
<b>N. Заголовок</b> — до 10 слов, до 80 символов.
2 коротких предложения. Без emoji, вступлений, рекламы, markdown, URL.`

func buildSingleNewsPrompt(rawNews string, number int, used []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Лента за 7 дней:\n%s\n\n", rawNews)
	if number == foreignNewsItemNum {
		b.WriteString("Нужен пункт <b>6.</b> — одна важная ЗАРУБЕЖНАЯ новость (не про Россию).\n")
	} else {
		fmt.Fprintf(&b, "Нужен пункт <b>%d.</b> — новость про Россию (РКН, Госдума, VPN, Telegram, Яндекс, рунет).\n", number)
	}
	if len(used) > 0 {
		b.WriteString("Уже выбраны темы (не повторяй): ")
		b.WriteString(strings.Join(used, "; "))
		b.WriteByte('\n')
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
	reNum := regexp.MustCompile(`<b>\s*\d{1,2}\.\s`)
	return reNum.ReplaceAllString(body, fmt.Sprintf("<b>%d. ", number))
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

	result, err := client.Models.GenerateContent(ctx, cfg.GeminiModel, contents, config)
	if err != nil {
		return "", "", fmt.Errorf("generate content: %w", err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return "", "", fmt.Errorf("пустой ответ от Gemini")
	}

	reason := genai.FinishReasonUnspecified
	if len(result.Candidates) > 0 {
		reason = result.Candidates[0].FinishReason
	}
	if reason == genai.FinishReasonMaxTokens {
		log.Printf("Gemini: ответ обрезан лимитом токенов (MAX_TOKENS)")
	}
	return text, reason, nil
}
