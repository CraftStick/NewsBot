package gemini

import (
	"sync"
	"time"
)

var (
	quotaMu  sync.Mutex
	quotaHit time.Time
)

var (
	usedDay string
	used    int
)

func pacificDay(t time.Time) string {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("PST", -8*3600)
	}
	return t.In(loc).Format("2006-01-02")
}

// noteCall считает запросы к Gemini за текущие сутки квоты (остаток Google не отдаёт).
func noteCall() {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	if d := pacificDay(time.Now()); d != usedDay {
		usedDay, used = d, 0
	}
	used++
}

// UsedToday — запросов к Gemini с начала суток квоты (память, обнуляется при рестарте).
func UsedToday() int {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	if pacificDay(time.Now()) != usedDay {
		return 0
	}
	return used
}

func noteQuota() { quotaMu.Lock(); quotaHit = time.Now(); quotaMu.Unlock() }

// QuotaResetAt — когда сбросится дневная квота, если с начала текущего окна она
// упиралась в лимит (free tier сбрасывается в полночь по Тихоокеанскому времени).
// shortcut: факт исчерпания хранится в памяти, после рестарта забывается.
func QuotaResetAt(now time.Time) (time.Time, bool) {
	quotaMu.Lock()
	hit := quotaHit
	quotaMu.Unlock()
	if hit.IsZero() {
		return time.Time{}, false
	}
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("PST", -8*3600)
	}
	h := hit.In(loc)
	reset := time.Date(h.Year(), h.Month(), h.Day()+1, 0, 0, 0, 0, loc)
	return reset, now.Before(reset)
}
