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
		// «Суд» и «мошенничество» без цифрового контекста — обычная криминальная
		// сводка: так в дайджест попал приговор основателю iSpring.
		{"Основателя iSpring Юрия Ускова оправдали", "Верховный суд Марий Эл отменил приговор по делу о мошенничестве", relevanceNone},
		{"Мошенники массово рассылают вредоносные приложения", "", relevanceTopic},
		{"Суд запретил компании использовать бренд", "Спор двух фирм о названии", relevanceNone},
		// Корпоративный спор про игровой чит: «досудебная претензия» плюс
		// «сервис» давали тему, хотя дайджест не про игры.
		{"Valve направила досудебную претензию разработчикам скинченджера для CS2", "Сервис закроют", relevanceNone},
		// Но блокировка той же платформы — наша новость: стоп-лист не перебивает
		// однозначные тематические слова.
		{"Роскомнадзор заблокировал Steam в России", "", relevanceTopic},
	}
	for _, c := range cases {
		if got := articleRelevance(c.title, c.summary); got != c.want {
			t.Errorf("%q: релевантность %d, ожидали %d", c.title, got, c.want)
		}
	}
}

// Ключевое слово ищется с начала слова: иначе «иск» находится в «поиске»,
// «рисках» и «диске», и статья про риски рынка попадает в дайджест.
func TestKeywordWordBoundary(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"поиск уязвимостей", "риски для рынка", "обзор жёсткого диска"} {
		if matchesKeyword(text, "иск") {
			t.Errorf("%q: «иск» не должен матчиться внутри слова", text)
		}
	}
	for _, text := range []string{"иск к оператору", "подали иски", "суд принял иск"} {
		if !matchesKeyword(text, "иск") {
			t.Errorf("%q: «иск» должен находиться", text)
		}
	}
	if articleRelevance("Аналитики оценили риски для онлайн-сервисов", "") == relevanceTopic {
		t.Fatal("статья про риски рынка не должна считаться темой дайджеста")
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
