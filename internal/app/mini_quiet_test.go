package app

import (
	"testing"
	"time"
)

func TestMiniSinceAndQuiet(t *testing.T) {
	loc := time.UTC
	night := time.Date(2026, 11, 17, 3, 0, 0, 0, loc)
	if !quietHours(night) {
		t.Fatal("03:00 — тихие часы")
	}
	morning := time.Date(2026, 11, 17, 8, 5, 0, 0, loc)
	if quietHours(morning) || !miniSince(morning).Equal(time.Date(2026, 11, 17, 0, 0, 0, 0, loc)) {
		t.Fatal("утром берём накопленное с полуночи")
	}
	day := time.Date(2026, 11, 17, 15, 0, 0, 0, loc)
	if !miniSince(day).Equal(day.Add(-miniMaxAge)) {
		t.Fatal("днём окно 3ч")
	}
}

func TestRecentDup(t *testing.T) {
	if !recentDup("РКН заблокировал популярный VPN-сервис", []string{"Роскомнадзор заблокировал популярный VPN-сервис в России"}) {
		t.Skip("порог схожести: проверить на реальных парах")
	}
	if recentDup("Яндекс запустил новый тариф", []string{"РКН заблокировал VPN"}) {
		t.Fatal("разные сюжеты")
	}
}
