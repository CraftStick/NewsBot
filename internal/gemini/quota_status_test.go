package gemini

import (
	"testing"
	"time"
)

func TestQuotaResetAt(t *testing.T) {
	quotaHit = time.Time{}
	if _, ok := QuotaResetAt(time.Now()); ok {
		t.Fatal("без сбоя квота не должна считаться исчерпанной")
	}
	noteQuota()
	reset, ok := QuotaResetAt(time.Now())
	if !ok || !reset.After(time.Now()) || reset.Sub(time.Now()) > 25*time.Hour {
		t.Fatalf("сброс вне суток: %v %v", reset, ok)
	}
	if _, ok := QuotaResetAt(reset.Add(time.Minute)); ok {
		t.Fatal("после сброса квота свободна")
	}
}
