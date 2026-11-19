package telegram

import (
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"treesheild-newsbot/internal/gemini"
	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/settings"
)

var startedAt = time.Now()

const skippedKeep = 30

var (
	skipMu  sync.Mutex
	skipped []string
)

// NoteSkipped запоминает отсеянный пост (последние skippedKeep) для /skipped.
func NoteSkipped(reason, title string) {
	skipMu.Lock()
	defer skipMu.Unlock()
	skipped = append(skipped, fmt.Sprintf("%s · %s — %s", time.Now().Format("02.01 15:04"), reason, title))
	if len(skipped) > skippedKeep {
		skipped = skipped[1:]
	}
}

func skippedText() string {
	skipMu.Lock()
	defer skipMu.Unlock()
	if len(skipped) == 0 {
		return "С последнего запуска ничего не отсеяно."
	}
	var b strings.Builder
	b.WriteString("<b>Отсеяно (новые внизу)</b>\n")
	for _, s := range skipped {
		b.WriteString(html.EscapeString(s) + "\n")
	}
	return b.String()
}

// statusText — сводка для /status: источники, Gemini, очередь, аптайм.
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
	gem := "🟢 Gemini: квота в порядке"
	if reset, ok := gemini.QuotaResetAt(time.Now()); ok {
		msk, _ := time.LoadLocation("Europe/Moscow")
		gem = "🔴 Gemini: дневная квота исчерпана, сброс ~" + reset.In(msk).Format("15:04") + " МСК"
	}
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*.json"))
	up := time.Since(startedAt).Round(time.Minute)
	return fmt.Sprintf("<b>Источники</b>\n%s\n%s\nЖдут решения: %d\nБот работает: %s", b.String(), gem, len(files), up)
}
