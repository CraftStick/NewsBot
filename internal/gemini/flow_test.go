package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"google.golang.org/genai"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
)

func item(n int) string {
	return fmt.Sprintf("<b>%d. Заголовок новости номер %d</b>\nПервое предложение достаточно длинное для проверки. Второе тоже длинное предложение.", n, n)
}

func reply(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{
		Content: &genai.Content{Parts: []*genai.Part{{Text: text}}}, FinishReason: genai.FinishReasonStop}}}
}

func stub(t *testing.T, f func(model, system, user string) (string, error)) {
	t.Helper()
	oldGen, oldGap := generate, geminiMinRequestGap
	geminiMinRequestGap = 0
	generate = func(_ context.Context, _ *genai.Client, model string, contents []*genai.Content, c *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		text, err := f(model, c.SystemInstruction.Parts[0].Text, contents[0].Parts[0].Text)
		if err != nil {
			return nil, err
		}
		return reply(text), nil
	}
	t.Cleanup(func() { generate, geminiMinRequestGap = oldGen, oldGap })
}

func TestShortBatchIsCompletedOneByOne(t *testing.T) {
	var mu sync.Mutex
	singles := 0
	stub(t, func(model, system, user string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if system == systemPromptSingle {
			singles++
			return item(6), nil
		}
		return "Вот новости:\n\n" + strings.Join([]string{item(1), item(2), item(3), item(4), item(5)}, "\n\n"), nil
	})
	body, err := GenerateDigest(context.Background(), config.Config{GeminiAPIKey: "k", GeminiModel: "main"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := digest.CountNewsItems(body); got != 6 || singles != 1 {
		t.Fatalf("пунктов %d (ждали 6), одиночных запросов %d (ждали 1)\n%s", got, singles, body)
	}
}

func TestFallbackModelAfterTwoOverloads(t *testing.T) {
	var models []string
	stub(t, func(model, system, user string) (string, error) {
		models = append(models, model)
		if model == "main" {
			return "", errors.New("Error 503, high demand, UNAVAILABLE")
		}
		return strings.Join([]string{item(1), item(2), item(3), item(4), item(5), item(6)}, "\n\n"), nil
	})
	cfg := config.Config{GeminiAPIKey: "k", GeminiModel: "main", GeminiFallbackModel: "lite"}
	if _, err := GenerateDigest(context.Background(), cfg, nil, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Join(models, ",") != "main,main,lite" {
		t.Fatalf("порядок моделей: %v", models)
	}
}
