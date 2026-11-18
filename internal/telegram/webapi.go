package telegram

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"treesheild-newsbot/internal/config"
)

// PendingItem — отправленный в личку пост, ожидающий решения.
type PendingItem struct {
	Key      string    `json:"key"`
	HTML     string    `json:"html"`
	HasImage bool      `json:"has_image"`
	At       time.Time `json:"at"`
}

// ListPending — очередь, новые первыми (для мини-апки).
func ListPending() []PendingItem {
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*.json"))
	out := []PendingItem{}
	for _, f := range files {
		key := filepath.Base(f)
		key = key[:len(key)-len(".json")]
		b, err := os.ReadFile(f)
		st, err2 := os.Stat(f)
		var p pendingPost
		if err != nil || err2 != nil || !pendingKeyRE.MatchString(key) || json.Unmarshal(b, &p) != nil {
			continue
		}
		_, ierr := os.Stat(imagePath(key, p))
		out = append(out, PendingItem{key, p.HTML, p.Image != "" && ierr == nil, st.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

func imagePath(key string, p pendingPost) string {
	if p.Image != "" {
		return p.Image
	}
	return filepath.Join(pendingDir, key+".jpg")
}

// PendingImage — путь к фото заготовки или "".
func PendingImage(key string) string {
	if !pendingKeyRE.MatchString(key) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(pendingDir, key+".json"))
	var p pendingPost
	if err != nil || json.Unmarshal(b, &p) != nil || p.Image == "" {
		return ""
	}
	return p.Image
}

// EditPending заменяет текст заготовки (ключ остаётся прежним).
func EditPending(key, html string) error {
	publishMu.Lock()
	defer publishMu.Unlock()
	if !pendingKeyRE.MatchString(key) || html == "" || len([]rune(html)) > 4096 {
		return errors.New("неверные данные")
	}
	path := filepath.Join(pendingDir, key+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return errors.New("пост уже опубликован или устарел")
	}
	var p pendingPost
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	p.HTML = html
	b, _ = json.Marshal(p)
	return os.WriteFile(path, b, 0o600)
}

func PublishPending(cfg config.Config, key string) error { return publishPending(cfg, key) }
func SkipPending(key string)                             { skipPending(key) }
