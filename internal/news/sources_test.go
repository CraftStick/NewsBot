package news

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSources(t *testing.T) {
	oldFeeds, oldTopic := feedSources, topicKeywords
	defer func() { feedSources, topicKeywords = oldFeeds, oldTopic }()

	if err := LoadSources(filepath.Join(t.TempDir(), "нет.json"), false); err != nil {
		t.Fatalf("отсутствующий файл не ошибка: %v", err)
	}
	if err := LoadSources(filepath.Join(t.TempDir(), "нет.json"), true); err == nil {
		t.Fatal("явно указанный отсутствующий файл должен дать ошибку")
	}
	p := filepath.Join(t.TempDir(), "s.json")
	_ = os.WriteFile(p, []byte(`{"feeds":[{"name":"A","url":"https://a/rss"}],"topic":["vpn"]}`), 0o600)
	if err := LoadSources(p, true); err != nil {
		t.Fatal(err)
	}
	if len(feedSources) != 1 || feedSources[0].Name != "A" || len(topicKeywords) != 1 {
		t.Fatalf("не применилось: %v %v", feedSources, topicKeywords)
	}
	_ = os.WriteFile(p, []byte(`{"feeds":[{"name":"","url":"x"}]}`), 0o600)
	if err := LoadSources(p, true); err == nil {
		t.Fatal("кривая лента должна давать ошибку")
	}
}
