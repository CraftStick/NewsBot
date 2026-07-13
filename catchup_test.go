package main

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestLastMissedSlot(t *testing.T) {
	t.Parallel()
	sched, err := cron.ParseStandard("0 18 * * 5") // пятница 18:00
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	at := func(s string) time.Time {
		tm, err := time.Parse("2006-01-02 15:04", s)
		if err != nil {
			t.Fatalf("time %q: %v", s, err)
		}
		return tm
	}

	// Только что отправили в пятницу, сейчас среда — пропусков нет.
	if _, found := lastMissedSlot(sched, at("2026-07-10 18:01"), at("2026-07-15 12:00")); found {
		t.Fatal("не должно быть пропущенного слота сразу после отправки")
	}

	// Отправляли в прошлую пятницу, эту бот проспал (лежал в 18:00, встал в 19:00).
	slot, found := lastMissedSlot(sched, at("2026-07-03 18:01"), at("2026-07-10 19:00"))
	if !found || !slot.Equal(at("2026-07-10 18:00")) {
		t.Fatalf("ожидали пропуск 2026-07-10 18:00, получили %v (found=%v)", slot, found)
	}

	// Долгий простой: досылаем самый свежий пропущенный слот, не старый.
	slot, found = lastMissedSlot(sched, at("2026-06-26 18:01"), at("2026-07-13 12:00"))
	if !found || !slot.Equal(at("2026-07-10 18:00")) {
		t.Fatalf("ожидали последний слот 2026-07-10 18:00, получили %v (found=%v)", slot, found)
	}
}
