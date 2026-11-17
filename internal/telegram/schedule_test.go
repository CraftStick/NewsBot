package telegram

import (
	"testing"
	"time"
)

func TestLaterTime(t *testing.T) {
	loc := time.FixedZone("msk", 3*3600)
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, loc)
	if got, _ := laterTime("1h", now, loc); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("1h: %v", got)
	}
	if got, _ := laterTime("am", now, loc); got.Day() != 18 || got.Hour() != 9 {
		t.Fatalf("am днём → завтра 09:00, получили %v", got)
	}
	early := time.Date(2026, 11, 17, 6, 0, 0, 0, loc)
	if got, _ := laterTime("am", early, loc); got.Day() != 17 || got.Hour() != 9 {
		t.Fatalf("am ночью → сегодня 09:00, получили %v", got)
	}
	if _, ok := laterTime("x", now, loc); ok {
		t.Fatal("неизвестный вариант принят")
	}
}
