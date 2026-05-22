package main

import (
	"strings"
	"testing"
	"time"
)

func TestTextImpliesOlderThan(t *testing.T) {
	t.Parallel()
	loc := time.UTC
	since := time.Date(2026, 5, 9, 0, 0, 0, 0, loc)

	if !textImpliesOlderThan("РКН в феврале 2026", "", since) {
		t.Fatal("expected february 2026 to be stale in may")
	}
	if textImpliesOlderThan("VPN и блокировки на этой неделе", "", since) {
		t.Fatal("no explicit old date should pass")
	}
}

func TestGoogleNewsWhen7dURL(t *testing.T) {
	t.Parallel()
	u := googleNewsWhen7dURL("https://news.google.com/rss/search?q=VPN&hl=ru")
	if !strings.Contains(u, "when:7d") {
		t.Fatalf("missing when:7d: %s", u)
	}
}
