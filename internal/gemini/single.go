package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/news"
)

const singlePostArticles = 15

// GenerateSingle пишет один пост (пункт «1.») из лучших статей ленты.
func GenerateSingle(ctx context.Context, cfg config.Config, articles []news.Article, published []string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: cfg.GeminiAPIKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}
	budget := &requestBudget{left: geminiRunRequestBudget, minGap: geminiMinRequestGap}
	return generateSingleNewsItem(ctx, client, cfg, news.BuildDigestPrompt(articles, singlePostArticles), 1, published, budget)
}
