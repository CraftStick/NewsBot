package telegram

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
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
	if reason == "дубль" {
		stat.dup++
	} else {
		stat.off++
	}
	skipped = append(skipped, fmt.Sprintf("%s · %s — %s", time.Now().Format("02.01 15:04"), reason, title))
	if len(skipped) > skippedKeep {
		skipped = skipped[1:]
	}
}

var stat struct {
	checked, sent, dup, off int
	last                    time.Time
}

// NoteChecked/NoteSent — счётчики мини-постов с запуска для /skipped.
func NoteChecked() { skipMu.Lock(); stat.checked++; stat.last = time.Now(); skipMu.Unlock() }
func NoteSent()    { skipMu.Lock(); stat.sent++; skipMu.Unlock() }

func skippedText() string {
	skipMu.Lock()
	defer skipMu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "<b>С запуска</b>\nНовых: %d · отправлено: %d · отсеяно: %d\n(дубли %d, не по теме %d)\n",
		stat.checked, stat.sent, stat.dup+stat.off, stat.dup, stat.off)
	fmt.Fprintf(&b, "👎 Удалено вами как не понравившееся: %d\n", dislikedCount())
	if len(skipped) == 0 {
		b.WriteString("\nОтсеивать пока нечего.")
		return b.String()
	}
	b.WriteString("\n<b>Отсеяно (новые внизу)</b>\n")
	for _, s := range skipped {
		b.WriteString(html.EscapeString(s) + "\n")
	}
	return b.String()
}

// statusText — короткая сводка для /status.
func statusText() string {
	var b strings.Builder
	b.WriteString("<b>Источники</b>\n")
	for _, s := range news.SourceStatus() {
		icon, note := "🟢", "ждёт опроса"
		switch {
		case settings.IsOff(s.ID):
			icon, note = "⚪", "выключен"
		case s.Fails > 0:
			icon, note = "🔴", fmt.Sprintf("сбоев: %d", s.Fails)
		case !s.LastOK.IsZero():
			note = s.LastOK.Format("15:04")
		}
		fmt.Fprintf(&b, "%s %s · %s\n", icon, html.EscapeString(s.ID), note)
	}
	gem := "🟢 квота в порядке"
	if reset, ok := gemini.QuotaResetAt(time.Now()); ok {
		msk, _ := time.LoadLocation("Europe/Moscow")
		gem = "🔴 квота кончилась, сброс ~" + reset.In(msk).Format("15:04") + " МСК"
	}
	fmt.Fprintf(&b, "\n<b>Gemini</b>\n%s · запросов за сутки: %d\n", gem, gemini.UsedToday())
	b.WriteString("\n<b>Ожидают</b>\n")
	ps := pendingList()
	if len(ps) == 0 {
		b.WriteString("пусто\n")
	}
	for _, p := range ps {
		b.WriteString("• " + html.EscapeString(p.title) + "\n")
	}
	if len(ps) > 0 {
		b.WriteString("→ кнопка «Ожидают»\n")
	}
	fmt.Fprintf(&b, "\nАптайм: %s", time.Since(startedAt).Round(time.Minute))
	return b.String()
}

type pendItem struct{ key, title, html string }

var tagRE = regexp.MustCompile(`<[^>]*>`)

// pendingList — заготовки, ждущие «Опубликовать/Пропустить»; title — первая строка без тегов.
func pendingList() []pendItem {
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*.json"))
	var out []pendItem
	for _, f := range files {
		raw, err := os.ReadFile(f)
		var p pendingPost
		if err != nil || json.Unmarshal(raw, &p) != nil {
			continue
		}
		t := strings.TrimSpace(strings.SplitN(html.UnescapeString(tagRE.ReplaceAllString(p.HTML, "")), "\n", 2)[0])
		if r := []rune(t); len(r) > 80 {
			t = string(r[:80]) + "…"
		}
		out = append(out, pendItem{strings.TrimSuffix(filepath.Base(f), ".json"), t, p.HTML})
	}
	return out
}
