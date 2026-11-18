package telegram

import (
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"time"

	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/settings"
)

var startedAt = time.Now()

// statusText — сводка для /status: источники, очередь постов, аптайм.
func statusText() string {
	var b strings.Builder
	for _, s := range news.SourceStatus() {
		icon, note := "🟢", "ещё не опрашивался"
		switch {
		case settings.IsOff(s.ID):
			icon, note = "⚪", "выключен"
		case s.Fails > 0:
			icon, note = "🔴", fmt.Sprintf("сбоев подряд: %d", s.Fails)
		case !s.LastOK.IsZero():
			note = "ок, " + s.LastOK.Format("15:04")
		}
		fmt.Fprintf(&b, "%s %s — %s\n", icon, html.EscapeString(s.ID), note)
	}
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*.json"))
	up := time.Since(startedAt).Round(time.Minute)
	return fmt.Sprintf("<b>Источники</b>\n%s\nЖдут решения: %d\nБот работает: %s", b.String(), len(files), up)
}
