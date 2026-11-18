package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func sign(token string, kv map[string]string) string {
	q := url.Values{}
	for k, v := range kv {
		q.Set(k, v)
	}
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(token))
	h := hmac.New(sha256.New, mac.Sum(nil))
	// порядок: ключи по алфавиту
	h.Write([]byte("auth_date=" + kv["auth_date"] + "\nuser=" + kv["user"]))
	q.Set("hash", hex.EncodeToString(h.Sum(nil)))
	return q.Encode()
}

func TestCheckInitData(t *testing.T) {
	now := time.Now()
	kv := map[string]string{"auth_date": strconv.FormatInt(now.Unix(), 10), "user": `{"id":42}`}
	d := sign("tok", kv)
	if id, err := CheckInitData(d, "tok", now); err != nil || id != 42 {
		t.Fatal(id, err)
	}
	if _, err := CheckInitData(d, "other", now); err == nil {
		t.Fatal("чужой токен принят")
	}
	if _, err := CheckInitData(d, "tok", now.Add(48*time.Hour)); err == nil {
		t.Fatal("устаревшая сессия принята")
	}
	if _, err := CheckInitData("user=%7B%22id%22%3A1%7D", "tok", now); err == nil {
		t.Fatal("без подписи принято")
	}
}
