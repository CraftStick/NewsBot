package gemini

import (
	"testing"

	"treesheild-newsbot/internal/config"
)

func TestFallbackAfterTwoOverloads(t *testing.T) {
	cfg := config.Config{GeminiModel: "main", GeminiFallbackModel: "lite"}
	b := &requestBudget{}
	if b.noteOverload(cfg) || b.model(cfg) != "main" {
		t.Fatal("после одной перегрузки остаёмся на основной")
	}
	if !b.noteOverload(cfg) || b.model(cfg) != "lite" {
		t.Fatal("после второй должны перейти на запасную")
	}
	off := config.Config{GeminiModel: "main"}
	b = &requestBudget{}
	b.noteOverload(off)
	if b.noteOverload(off) || b.model(off) != "main" {
		t.Fatal("без запасной модели переключения нет")
	}
}
