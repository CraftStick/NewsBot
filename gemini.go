package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

func generateDigest(ctx context.Context, cfg Config, rawNews string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{{Text: rawNews}},
		},
	}

	temp := float32(0.35)
	maxOut := int32(1800)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		Temperature:     &temp,
		MaxOutputTokens: maxOut,
	}

	result, err := client.Models.GenerateContent(ctx, cfg.GeminiModel, contents, config)
	if err != nil {
		return "", fmt.Errorf("generate content: %w", err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return "", fmt.Errorf("пустой ответ от Gemini")
	}
	return text, nil
}
