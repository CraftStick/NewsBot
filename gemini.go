package main

import (
	"context"
	"errors"
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
	// Потолок запросов к Gemini на один прогон. Без него каскад ретраев
	// (4 попытки × 5 повторов, затем 6 пунктов × 3 лимита токенов) выжирает
	// дневную квоту бесплатного тарифа за один-единственный дайджест.
	geminiRunRequestBudget = 8

	// Лимиты для фото-режима. Подпись Telegram жёстко ограничена, а сжатие в
	// format.go режет пункты до первого предложения — чтобы вторая фраза дожила
	// до отправки, пункт должен влезать в лимит сразу (см. TestCaptionBudget).
	captionHeadingMaxChars = 50
	captionItemMaxChars    = 138
)

// requestBudget — общий на прогон счётчик запросов к Gemini.
type requestBudget struct {
	left int
}

func (b *requestBudget) take() error {
	if b.left <= 0 {
		return errBudgetExhausted
	}
	b.left--
	return nil
}

var (
	errBudgetExhausted = fmt.Errorf(
		"исчерпан бюджет запросов к Gemini на один прогон (%d) — остаток дневной квоты сохранён",
		geminiRunRequestBudget)

	// errEmptyResponse — модель вернула пустой текст (упёрлась в MaxOutputTokens
	// или фильтры). Повторять тот же запрос бессмысленно, надо менять параметры.
	errEmptyResponse = errors.New("пустой ответ от Gemini")
)

// thinkingConfigFor выключает «мышление» у 2.5-flash: оно включено по умолчанию
// и тратит те же MaxOutputTokens, из-за чего модель упирается в лимит и отдаёт
// пустой текст. Для дайджеста рассуждения не нужны. У pro-моделей нулевой
// бюджет запрещён, поэтому там конфиг не трогаем.
func thinkingConfigFor(model string) *genai.ThinkingConfig {
	if !strings.Contains(strings.ToLower(model), "2.5-flash") {
		return nil
	}
	zero := int32(0)
	return &genai.ThinkingConfig{ThinkingBudget: &zero}
}

var singleNewsNumRE = regexp.MustCompile(`<b>\s*\d{1,2}\.\s`)

func generateDigest(ctx context.Context, cfg Config, articles []Article, published []string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}

	fullPrompt := buildNewsDigestPrompt(articles, 0)
	if len(published) > 0 {
		fullPrompt += "\n\nСТОП-ЛИСТ. Эти темы уже выходили в прошлых выпусках. Не бери их снова ни в каком виде: " +
			"ни то же событие под другой формулировкой, ни его продолжение или уточнение. Нужны другие сюжеты:\n- " +
			strings.Join(published, "\n- ")
	}
	if cfg.PhotoEnabled {
		fullPrompt += fmt.Sprintf(
			"\n\nДайджест пойдёт в подпись к фото — там жёсткий лимит места. Пиши ОЧЕНЬ компактно: "+
				"заголовок до %d символов, под ним 2 коротких, но ОБЯЗАТЕЛЬНО законченных предложения. "+
				"Весь пункт целиком (заголовок плюс оба предложения) — не длиннее %d символов. "+
				"Оба предложения обязательны, и второе должно нести НОВЫЙ факт, а не пересказ заголовка: "+
				"лучше две плотных фразы по 40 символов, чем одна пустая.",
			captionHeadingMaxChars, captionItemMaxChars)
	}
	budget := &requestBudget{left: geminiRunRequestBudget}

	body, err := generateDigestBatch(ctx, client, cfg, fullPrompt, budget)
	if err == nil {
		return body, nil
	}
	if isQuotaExhausted(err) {
		return "", errQuotaExhausted // последовательный проход упрётся в ту же дневную квоту
	}
	if errors.Is(err, errBudgetExhausted) {
		return "", err // по одной новости — это ещё до 90 запросов, дневной квоты не хватит
	}
	log.Printf("Пакетная генерация не удалась (%v), пробуем по одной новости…", err)

	return generateDigestSequential(ctx, client, cfg, buildNewsDigestPrompt(articles, geminiSequentialArticles), published, budget)
}

func generateDigestBatch(
	ctx context.Context,
	client *genai.Client,
	cfg Config,
	userText string,
	budget *requestBudget,
) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= geminiMaxBatchAttempts; attempt++ {
		prompt := userText
		if attempt > 1 {
			prompt += "\n\nПредыдущий ответ не подошёл. Верни все 6 пунктов (1–5 Россия, 6 — зарубеж). Заголовки до 10 слов. В каждом — 2 коротких предложения."
		}

		body, reason, err := callGemini(ctx, client, cfg, systemPrompt, prompt, attempt, geminiMaxOutputTokens, budget)
		if err != nil {
			lastErr = err
			if errors.Is(err, errEmptyResponse) || isGeminiRetryable(err) {
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

func generateDigestSequential(
	ctx context.Context,
	client *genai.Client,
	cfg Config,
	feed string,
	published []string,
	budget *requestBudget,
) (string, error) {
	var parts []string
	// Темы прошлых выпусков идут в тот же список «не повторяй», что и уже
	// выбранные в этом прогоне.
	usedTitles := append([]string(nil), published...)

	for n := 1; n <= requiredNewsItems; n++ {
		body, err := generateSingleNewsItem(ctx, client, cfg, feed, n, usedTitles, budget)
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
	budget *requestBudget,
) (string, error) {
	prompt := buildSingleNewsPrompt(feed, number, used)
	tokens := []int32{geminiSingleOutputTokens, 1800, 2400}
	var lastErr error

	for i, maxOut := range tokens {
		extra := ""
		if i > 0 {
			extra = "\n\nОтветь только одним пунктом: <b>N. Заголовок</b> и 2 коротких предложения."
		}
		body, reason, err := callGemini(ctx, client, cfg, systemPromptSingle, prompt+extra, i+1, maxOut, budget)
		if err != nil {
			if errors.Is(err, errEmptyResponse) || isGeminiRetryable(err) {
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
2 коротких предложения. Без emoji, вступлений, рекламы, markdown, URL.
Бери самое свежее по дате состояние темы, не устаревшую формулировку («готовят/планируют», если событие уже случилось).`

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

func normalizeSingleNewsBlock(body string, number int) string {
	body = strings.TrimSpace(body)
	if countNewsItems(body) == 0 {
		if !strings.HasPrefix(body, "<b>") {
			body = fmt.Sprintf("<b>%d. %s</b>\n%s", number, body, "")
		}
	}
	return singleNewsNumRE.ReplaceAllString(body, fmt.Sprintf("<b>%d. ", number))
}

// errQuotaExhausted — понятное сообщение вместо простыни от Gemini при исчерпании
// дневной квоты (её всё равно бесполезно ретраить в пределах запуска).
var errQuotaExhausted = errors.New(
	"дневной лимит запросов Gemini исчерпан (бесплатный тариф — 20 запросов в сутки). " +
		"Сбросится в течение суток; для больших объёмов включите биллинг в Google AI Studio")

// isQuotaExhausted — именно ДНЕВНАЯ квота (не поминутный рейт-лимит): ретраи не помогут.
func isQuotaExhausted(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "perday") ||
		strings.Contains(s, "per day") ||
		strings.Contains(s, "free_tier_requests")
}

func isGeminiRetryable(err error) bool {
	if err == nil {
		return false
	}
	if isQuotaExhausted(err) {
		return false // дневную квоту ретраить бессмысленно — только жжём остаток
	}
	if errors.Is(err, errBudgetExhausted) || errors.Is(err, errEmptyResponse) {
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
	budget *requestBudget,
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
		ThinkingConfig:  thinkingConfigFor(cfg.GeminiModel),
	}

	var lastErr error
	for try := 1; try <= geminiMaxAPIRetries; try++ {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		if err := budget.take(); err != nil {
			if lastErr != nil {
				return "", "", fmt.Errorf("%w (последняя ошибка: %v)", err, lastErr)
			}
			return "", "", err
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
			// Повтор того же запроса ничего не изменит: пустой текст — это
			// упёршийся в MaxOutputTokens или фильтры вызов, а не сбой сети.
			// Меняют параметры выше по стеку, здесь просто не жжём квоту.
			return "", reason, fmt.Errorf("%w (finish=%s)", errEmptyResponse, reason)
		}
		if reason == genai.FinishReasonMaxTokens {
			log.Printf("Gemini: ответ обрезан (MAX_TOKENS, лимит %d)", maxOut)
		}
		return text, reason, nil
	}
	return "", "", lastErr
}
