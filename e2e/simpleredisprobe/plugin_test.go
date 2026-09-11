package simpleredisprobe

import (
	"context"
	"net/http"
	"testing"
)

// TestNewDoesNotDial proves New Inits only: a refusing Redis host still returns a handler.
func TestNewDoesNotDial(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler, err := New(context.Background(), next, &Config{Host: "127.0.0.1:1"}, "probe")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if handler == nil {
		t.Fatal("New returned nil handler")
	}
}
