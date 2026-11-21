package news

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// Идеи из RSS-to-Telegram-Bot: условный GET (ETag/Last-Modified), пауза после
// серии ошибок, повтор по IPv4 при отказах, похожих на бан IPv6-диапазона.

var ipv4Client = &http.Client{
	Timeout: httpClient.Timeout,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp4", addr)
		},
	},
}

// doGet выполняет запрос; при сетевой ошибке или 400/403/429/451 повторяет по IPv4.
func doGet(req *http.Request) (*http.Response, error) {
	resp, err := httpClient.Do(req)
	if err == nil && !retryInV4(resp.StatusCode) {
		return resp, nil
	}
	if req.Context().Err() != nil {
		return resp, err
	}
	if err == nil {
		_ = resp.Body.Close()
	}
	if resp2, err2 := ipv4Client.Do(req); err2 == nil {
		return resp2, nil
	}
	return httpClient.Do(req)
}

func retryInV4(code int) bool {
	return code == 400 || code == 403 || code == 429 || code == 451
}

type validator struct{ etag, lastMod string }

var (
	valMu sync.Mutex
	vals  = map[string]validator{}
)

// fetchFeedCond: на 304 возвращает (nil, nil) — лента не менялась.
func fetchFeedCond(ctx context.Context, parser *gofeed.Parser, url string) (*gofeed.Feed, error) {
	ctx, cancel := context.WithTimeout(ctx, feedTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", parser.UserAgent)
	valMu.Lock()
	v := vals[url]
	valMu.Unlock()
	if v.etag != "" {
		req.Header.Set("If-None-Match", v.etag)
	}
	if v.lastMod != "" {
		req.Header.Set("If-Modified-Since", v.lastMod)
	}
	resp, err := doGet(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	feed, err := parser.Parse(io.LimitReader(resp.Body, feedMaxBytes))
	if err == nil {
		valMu.Lock()
		vals[url] = validator{resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")}
		valMu.Unlock()
	}
	return feed, err
}

func fetchFeedCondRetry(ctx context.Context, parser *gofeed.Parser, url string) (*gofeed.Feed, error) {
	var lastErr error
	for attempt := 1; attempt <= rssFetchAttempts; attempt++ {
		feed, err := fetchFeedCond(ctx, parser, url)
		if err == nil {
			return feed, nil
		}
		lastErr = err
		if attempt < rssFetchAttempts && ctx.Err() == nil {
			time.Sleep(rssRetryDelay)
		}
	}
	return nil, lastErr
}

var (
	tryMu   sync.Mutex
	lastTry = map[string]time.Time{}
)

// backoffDelay: после 10 неудач подряд проверяем реже — вдвое за каждые 10, не дольше суток.
func backoffDelay(fails int) time.Duration {
	if fails < 10 {
		return 0
	}
	d := time.Minute << (fails / 10)
	if d > 24*time.Hour || d <= 0 {
		d = 24 * time.Hour
	}
	return d
}

// sourceDue сообщает, пора ли опрашивать источник, и отмечает попытку.
func sourceDue(id string, now time.Time) bool {
	miniFailsMu.Lock()
	fails := miniFails[id]
	miniFailsMu.Unlock()
	tryMu.Lock()
	defer tryMu.Unlock()
	if now.Sub(lastTry[id]) < backoffDelay(fails) {
		return false
	}
	lastTry[id] = now
	return true
}
