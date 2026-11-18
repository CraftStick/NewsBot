// Package settings — настройки, которые меняются из мини-апки без перезапуска.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type S struct {
	Off     []string `json:"off"`      // отключённые источники (id)
	NoQuiet bool     `json:"no_quiet"` // не копить новости в тихие часы 00–08
}

var (
	mu   sync.Mutex
	path = "settings.json"
	cur  S
	read bool
)

// SetPath задаёт файл настроек (по умолчанию рядом с рабочей папкой бота).
func SetPath(p string) { mu.Lock(); path, read = filepath.Clean(p), false; mu.Unlock() }

func load() {
	if read {
		return
	}
	read = true
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cur)
	}
}

func Get() S {
	mu.Lock()
	defer mu.Unlock()
	load()
	c := cur
	c.Off = append([]string(nil), c.Off...)
	return c
}

func Set(s S) error {
	mu.Lock()
	defer mu.Unlock()
	b, _ := json.Marshal(s)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	cur, read = s, true
	return nil
}

func IsOff(id string) bool {
	for _, o := range Get().Off {
		if o == id {
			return true
		}
	}
	return false
}
