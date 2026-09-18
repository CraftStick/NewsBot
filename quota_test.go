package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestQuotaDetection(t *testing.T) {
	t.Parallel()

	daily := errors.New("generate content: Error 429, Message: You exceeded your current quota. " +
		"Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, " +
		"limit: 20, model: gemini-2.5-flash ... quotaId:GenerateRequestsPerDayPerProjectPerModel-FreeTier")

	if !isQuotaExhausted(daily) {
		t.Fatal("дневная квота не распознана")
	}
	if isGeminiRetryable(daily) {
		t.Fatal("дневную квоту нельзя считать ретраебельной")
	}

	// Поминутный рейт-лимит (429, но НЕ дневной) — остаётся ретраебельным.
	perMinute := errors.New("Error 429: quotaId:GenerateRequestsPerMinutePerProjectPerModel-FreeTier")
	if isQuotaExhausted(perMinute) {
		t.Fatal("поминутный лимит ошибочно принят за дневной")
	}
	if !isGeminiRetryable(perMinute) {
		t.Fatal("поминутный 429 должен ретраиться")
	}
}

func TestRequestBudget(t *testing.T) {
	t.Parallel()

	b := &requestBudget{left: 2}
	if err := b.take(context.Background()); err != nil {
		t.Fatalf("первый запрос отклонён: %v", err)
	}
	if err := b.take(context.Background()); err != nil {
		t.Fatalf("второй запрос отклонён: %v", err)
	}
	err := b.take(context.Background())
	if !errors.Is(err, errBudgetExhausted) {
		t.Fatalf("ожидали исчерпание бюджета, получили %v", err)
	}
	if isGeminiRetryable(err) {
		t.Fatal("исчерпанный бюджет нельзя ретраить — иначе он бессмыслен")
	}
}

func TestEmptyResponseNotRetryable(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("%w (finish=MAX_TOKENS)", errEmptyResponse)
	if isGeminiRetryable(err) {
		t.Fatal("пустой ответ не транзиентный: повтор того же запроса жжёт квоту")
	}
}

func TestThinkingDisabledForFlash(t *testing.T) {
	t.Parallel()

	cfg := thinkingConfigFor("gemini-2.5-flash")
	if cfg == nil || cfg.ThinkingBudget == nil || *cfg.ThinkingBudget != 0 {
		t.Fatalf("для 2.5-flash мышление должно быть выключено, получили %+v", cfg)
	}
	if thinkingConfigFor("gemini-2.5-pro") != nil {
		t.Fatal("у pro нулевой бюджет запрещён — конфиг трогать нельзя")
	}
}

// Настоящая поминутная ошибка Gemini содержит имя метрики
// generate_content_free_tier_requests — то же, что и дневная. По нему её
// принимали за дневную квоту, и бот сдавался вместо 30-секундной паузы.
const realPerMinute429 = `generate content: Error 429, Message: You exceeded your current quota, please check your plan and billing details. ` +
	`* Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 5, model: gemini-2.5-flash
Please retry in 30.097593961s., Status: RESOURCE_EXHAUSTED, Details: [map[@type:type.googleapis.com/google.rpc.QuotaFailure ` +
	`violations:[map[quotaDimensions:map[location:global model:gemini-2.5-flash] quotaId:GenerateRequestsPerMinutePerProjectPerModel-FreeTier ` +
	`quotaMetric:generativelanguage.googleapis.com/generate_content_free_tier_requests quotaValue:5]]] ` +
	`map[@type:type.googleapis.com/google.rpc.RetryInfo retryDelay:30s]]`

func TestRealPerMinuteQuotaIsRetryable(t *testing.T) {
	t.Parallel()

	err := errors.New(realPerMinute429)
	if isQuotaExhausted(err) {
		t.Fatal("поминутный лимит принят за дневную квоту")
	}
	if !isGeminiRetryable(err) {
		t.Fatal("поминутный 429 обязан повторяться")
	}
	if d := serverRetryDelay(err); d < 30*time.Second || d > 31*time.Second {
		t.Fatalf("не прочитана пауза из ответа Gemini: %s", d)
	}
}

func TestServerRetryDelayFallback(t *testing.T) {
	t.Parallel()

	if d := serverRetryDelay(errors.New("RetryInfo retryDelay:45s")); d != 45*time.Second {
		t.Fatalf("retryDelay не разобран: %s", d)
	}
	if d := serverRetryDelay(errors.New("503 unavailable")); d != 0 {
		t.Fatalf("пауза без подсказки сервера должна быть нулевой: %s", d)
	}
}

// Между запросами выдерживается пауза: пачка быстрых повторов упиралась в
// поминутный лимит бесплатного тарифа.
func TestRequestBudgetPacing(t *testing.T) {
	t.Parallel()

	const gap = 60 * time.Millisecond
	b := &requestBudget{left: 3, minGap: gap}
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := b.take(context.Background()); err != nil {
			t.Fatalf("запрос %d отклонён: %v", i+1, err)
		}
	}
	if elapsed := time.Since(start); elapsed < 2*gap {
		t.Fatalf("три запроса прошли за %s, ожидали не меньше %s", elapsed, 2*gap)
	}
}
