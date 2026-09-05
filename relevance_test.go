package main

import (
	"testing"
	"time"
)

// Знакомый бренд в заголовке — ещё не новость дайджеста: такие статьи проходят
// вторым эшелоном и вытесняются тематическими.
func TestArticleRelevance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		title, summary string
		want           int
	}{
		{"Роскомнадзор начал блокировать протокол", "", relevanceTopic},
		{"В России выросли штрафы за утечки персональных данных", "", relevanceTopic},
		{"МТС улучшила мобильный интернет в регионах", "Обновлена сеть на юге области", relevanceEntity},
		{"StarLine интегрировал управление авто с умным домом Яндекса", "", relevanceEntity},
		{"«Яндекс» изучил вопросы школьников к «Алисе»", "", relevanceEntity},
		{"Вышел новый смартфон с тройной камерой", "", relevanceNone},
		{"Суд оштрафовал сервис за отказ удалять данные", "", relevanceTopic},
	}
	for _, c := range cases {
		if got := articleRelevance(c.title, c.summary); got != c.want {
			t.Errorf("%q: релевантность %d, ожидали %d", c.title, got, c.want)
		}
	}
}

// Пресс-релиз с высоким RU-приоритетом не должен обгонять тематическую новость:
// в промпт уходят первые maxArticlesInPrompt статей.
func TestTopicalBeatsPressRelease(t *testing.T) {
	t.Parallel()

	now := time.Now()
	articles := []Article{
		{Title: "МТС улучшила интернет в России", RUPriority: 5, Relevance: relevanceEntity, PublishedAt: now},
		{Title: "Хакеры слили базу пользователей", RUPriority: 0, Relevance: relevanceTopic, PublishedAt: now.Add(-48 * time.Hour)},
	}
	sortArticlesForPrompt(articles)

	if articles[0].Relevance != relevanceTopic {
		t.Fatalf("первым идёт пресс-релиз: %q", articles[0].Title)
	}
}
