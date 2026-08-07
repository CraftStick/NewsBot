package main

import (
	"errors"
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
