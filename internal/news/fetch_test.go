package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

const okFeed = `<?xml version="1.0"?><rss version="2.0"><channel><title>t</title><item><title>VPN</title></item></channel></rss>`

func TestFetchFeed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(okFeed)) })
	mux.HandleFunc("/403", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) })
	mux.HandleFunc("/endless", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel>`))
		chunk := []byte(strings.Repeat("x", 64<<10))
		for r.Context().Err() == nil {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	p := gofeed.NewParser()

	if f, err := fetchFeed(context.Background(), p, srv.URL+"/ok"); err != nil || len(f.Items) != 1 {
		t.Fatalf("ok: %v %v", f, err)
	}
	if _, err := fetchFeed(context.Background(), p, srv.URL+"/403"); err == nil {
		t.Fatal("403 должен быть ошибкой")
	}
	done := make(chan error, 1)
	go func() { _, err := fetchFeed(context.Background(), p, srv.URL+"/endless"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("бесконечная лента должна давать ошибку")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("лимит размера не сработал")
	}
}
