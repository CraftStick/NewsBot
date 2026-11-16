package gemini

import (
	"context"
	"testing"

	"treesheild-newsbot/internal/config"
)

func TestGenerateSingle(t *testing.T) {
	stub(t, func(model, system, user string) (string, error) { return item(1), nil })
	body, err := GenerateSingle(context.Background(), config.Config{GeminiAPIKey: "k", GeminiModel: "m"}, nil, nil)
	if err != nil || body == "" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}
