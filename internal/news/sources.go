package news

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

// sourcesFile — необязательный JSON рядом с бинарником: лента и ключевые слова
// без пересборки. Пустое/отсутствующее поле оставляет значение из кода.
type sourcesFile struct {
	Feeds   []struct{ Name, URL string } `json:"feeds"`
	Topic   []string                     `json:"topic"`
	Weak    []string                     `json:"weak"`
	Exclude []string                     `json:"exclude"`
	Digital []string                     `json:"digital"`
	Entity  []string                     `json:"entity"`
	RUBoost []string                     `json:"ru_boost"`
}

// LoadSources читает path. Нет файла — не ошибка, если explicit=false.
func LoadSources(path string, explicit bool) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && !explicit {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sources: %w", err)
	}
	var f sourcesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("sources %s: %w", path, err)
	}
	if len(f.Feeds) > 0 {
		feeds := feedSources[:0:0]
		for _, s := range f.Feeds {
			if strings.TrimSpace(s.Name) == "" || !strings.HasPrefix(s.URL, "http") {
				return fmt.Errorf("sources %s: лента %q: нужны name и http(s) url", path, s.Name)
			}
			feeds = append(feeds, struct{ Name, URL string }{s.Name, s.URL})
		}
		feedSources = feeds
	}
	for dst, src := range map[*[]string][]string{
		&topicKeywords: f.Topic, &weakTopicKeywords: f.Weak, &excludeKeywords: f.Exclude,
		&digitalContextKeywords: f.Digital, &entityKeywords: f.Entity, &ruBoostKeywords: f.RUBoost,
	} {
		if len(src) > 0 {
			*dst = src
		}
	}
	log.Printf("Источники из %s: лент %d", path, len(feedSources))
	return nil
}
