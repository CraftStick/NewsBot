package main

import (
	"errors"
	"fmt"
	"testing"
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
	if err := b.take(); err != nil {
		t.Fatalf("первый запрос отклонён: %v", err)
	}
	if err := b.take(); err != nil {
		t.Fatalf("второй запрос отклонён: %v", err)
	}
	err := b.take()
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
