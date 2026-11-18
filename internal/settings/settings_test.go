package settings

import (
	"path/filepath"
	"testing"
)

func TestSetGet(t *testing.T) {
	SetPath(filepath.Join(t.TempDir(), "s.json"))
	if IsOff("a") {
		t.Fatal("пусто")
	}
	if err := Set(S{Off: []string{"a"}, NoQuiet: true}); err != nil {
		t.Fatal(err)
	}
	SetPath(path) // перечитать с диска
	if !IsOff("a") || !Get().NoQuiet {
		t.Fatalf("%+v", Get())
	}
}
