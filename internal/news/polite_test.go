package news

import (
	"testing"
	"time"
)

func TestBackoffDelay(t *testing.T) {
	if backoffDelay(9) != 0 || backoffDelay(10) != 2*time.Minute || backoffDelay(10000) != 24*time.Hour {
		t.Fatal("backoffDelay")
	}
}

func TestRetryInV4(t *testing.T) {
	if !retryInV4(429) || retryInV4(200) || retryInV4(404) {
		t.Fatal("retryInV4")
	}
}
