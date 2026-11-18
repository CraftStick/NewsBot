// Package web — мини-апка Telegram: лента, статус, настройки. Доступ только владельцу (initData).
package web

import (
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/news"
	"treesheild-newsbot/internal/settings"
	"treesheild-newsbot/internal/telegram"
)

//go:embed index.html
var page []byte

var started = time.Now()

// CheckInitData проверяет подпись Telegram и возвращает id пользователя.
func CheckInitData(initData, token string, now time.Time) (int64, error) {
	q, err := url.ParseQuery(initData)
	if err != nil || q.Get("hash") == "" {
		return 0, errors.New("нет подписи")
	}
	var parts []string
	for k, v := range q {
		if k != "hash" {
			parts = append(parts, k+"="+v[0])
		}
	}
	sort.Strings(parts)
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(token))
	h := hmac.New(sha256.New, mac.Sum(nil))
	h.Write([]byte(strings.Join(parts, "\n")))
	if !hmac.Equal([]byte(hex.EncodeToString(h.Sum(nil))), []byte(q.Get("hash"))) {
		return 0, errors.New("подпись неверна")
	}
	if ts, _ := strconv.ParseInt(q.Get("auth_date"), 10, 64); now.Sub(time.Unix(ts, 0)) > 24*time.Hour {
		return 0, errors.New("сессия устарела")
	}
	var u struct{ ID int64 }
	if json.Unmarshal([]byte(q.Get("user")), &u) != nil || u.ID == 0 {
		return 0, errors.New("нет пользователя")
	}
	return u.ID, nil
}

// Handler собирает маршруты; owner — id чата владельца.
func Handler(cfg config.Config, owner int64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	api := func(path string, f func(w http.ResponseWriter, r *http.Request) any) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			id, err := CheckInitData(r.Header.Get("X-Init-Data"), cfg.TelegramToken, time.Now())
			if err != nil || id != owner {
				http.Error(w, "доступ запрещён", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			out := f(w, r)
			if e, ok := out.(error); ok {
				w.WriteHeader(http.StatusBadRequest)
				out = map[string]string{"error": e.Error()}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		})
	}
	api("/api/feed", func(http.ResponseWriter, *http.Request) any { return telegram.ListPending() })
	act := func(do func(key, html string) error) func(http.ResponseWriter, *http.Request) any {
		return func(_ http.ResponseWriter, r *http.Request) any {
			var in struct{ Key, HTML string }
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&in) != nil {
				return errors.New("неверный запрос")
			}
			if err := do(in.Key, in.HTML); err != nil {
				return err
			}
			return map[string]bool{"ok": true}
		}
	}
	api("/api/publish", act(func(k, _ string) error { return telegram.PublishPending(cfg, k) }))
	api("/api/skip", act(func(k, _ string) error { telegram.SkipPending(k); return nil }))
	api("/api/edit", act(telegram.EditPending))
	api("/api/status", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"sources": news.SourceStatus(), "pending": len(telegram.ListPending()),
			"uptime_min": int(time.Since(started).Minutes())}
	})
	api("/api/settings", func(_ http.ResponseWriter, r *http.Request) any {
		if r.Method == http.MethodPost {
			var s settings.S
			if json.NewDecoder(r.Body).Decode(&s) != nil {
				return errors.New("неверный запрос")
			}
			if err := settings.Set(s); err != nil {
				return err
			}
		}
		return map[string]any{"settings": settings.Get(), "sources": news.SourceIDs()}
	})
	// Фото открываются через <img>, который не умеет заголовки: подпись в query.
	mux.HandleFunc("/api/img", func(w http.ResponseWriter, r *http.Request) {
		id, err := CheckInitData(r.URL.Query().Get("d"), cfg.TelegramToken, time.Now())
		if p := telegram.PendingImage(r.URL.Query().Get("key")); err != nil || id != owner || p == "" {
			http.NotFound(w, r)
			return
		} else {
			http.ServeFile(w, r, p)
		}
	})
	return mux
}

// Run поднимает сервер; пустой addr — мини-апка выключена.
func Run(cfg config.Config, owner int64) {
	if cfg.WebAddr == "" {
		return
	}
	log.Printf("мини-апка: слушаю %s", cfg.WebAddr)
	srv := &http.Server{Addr: cfg.WebAddr, Handler: Handler(cfg, owner), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("мини-апка: %v", err)
	}
}
