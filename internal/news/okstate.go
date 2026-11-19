package news

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var okOnce sync.Once

// okPath — файл с временем последнего успешного опроса (переживает рестарт).
func okPath() string {
	if p := os.Getenv("SOURCE_STATE_FILE"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), ".source_ok")
	}
	return ".source_ok"
}

// loadOK и saveOK вызываются под miniFailsMu.
func loadOK() {
	okOnce.Do(func() {
		var m map[string]time.Time
		if b, err := os.ReadFile(okPath()); err == nil && json.Unmarshal(b, &m) == nil {
			for k, v := range m {
				miniLastOK[k] = v
			}
		}
	})
}

func saveOK() {
	if b, err := json.Marshal(miniLastOK); err == nil {
		_ = os.WriteFile(okPath(), b, 0o600) // не критично: потеряется только отметка времени
	}
}
