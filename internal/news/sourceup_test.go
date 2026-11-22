package news

import (
	"errors"
	"testing"
)

func TestOnSourceUp(t *testing.T) {
	var down, up int
	OnSourceDown, OnSourceUp = func(string) { down++ }, func(string) { up++ }
	defer func() { OnSourceDown, OnSourceUp = nil, nil }()
	const id = "test-source-up"
	noteSource(id, nil)
	for i := 0; i < miniDownAfter; i++ {
		noteSource(id, errors.New("502"))
	}
	noteSource(id, nil)
	noteSource(id, nil)
	if down != 1 || up != 1 {
		t.Fatalf("down=%d up=%d, ждали по одному", down, up)
	}
}
