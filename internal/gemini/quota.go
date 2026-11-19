package gemini

import (
	"sync"
	"time"
)

var (
	quotaMu  sync.Mutex
	quotaHit time.Time
)

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
