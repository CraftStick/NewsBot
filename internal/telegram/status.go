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
	fmt.Fprintf(&b, "<b>С запуска</b>: новых статей разобрано %d, отправлено %d, отсеяно %d (дубли %d, не по теме %d).\n",
		stat.checked, stat.sent, stat.dup+stat.off, stat.dup, stat.off)
	b.WriteString("Не по теме отсеивает Gemini, только у «Код Дурова»; дубль — тот же сюжет, что уже присылали.\n")
	if stat.last.IsZero() {
		b.WriteString("Новых статей пока не было.")
		return b.String()
	}
	if len(skipped) == 0 {
		b.WriteString("Всё, что пришло, прошло фильтры — отсеивать было нечего.")
		return b.String()
	}
	b.WriteString("\n<b>Отсеяно (новые внизу)</b>\n")
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
	gem += fmt.Sprintf("\nЗапросов к Gemini за сутки квоты: %d (Google остаток не сообщает; при исчерпании бот покажет время сброса)", gemini.UsedToday())
	pend := "нет"
	if ps := pendingList(); len(ps) > 0 {
		var l []string
		for _, p := range ps {
			l = append(l, "• "+html.EscapeString(p.title))
		}
		pend = fmt.Sprintf("%d — жми «Ожидают», чтобы посмотреть и решить\n%s", len(ps), strings.Join(l, "\n"))
	}
	up := time.Since(startedAt).Round(time.Minute)
	return fmt.Sprintf("<b>Источники</b>\n%s\n%s\n\n<b>Ждут решения:</b> %s\n\nБот работает: %s", b.String(), gem, pend, up)
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
