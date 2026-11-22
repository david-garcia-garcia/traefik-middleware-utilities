package ratelimitprobe

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"
)

// TestNewDoesNotDial proves New constructs both limiters without dialing: a refusing host still returns a handler.
func TestNewDoesNotDial(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	configs := map[string]*Config{
		"tokenbucket":   {Host: "127.0.0.1:1", Limiter: "tokenbucket", Rate: "0.5", Burst: "3"},
		"windowcounter": {Host: "127.0.0.1:1", Limiter: "windowcounter", Limit: "3", Window: "60s"},
	}
	for name, cfg := range configs {
		handler, err := New(context.Background(), next, cfg, "probe")
		if err != nil {
			t.Fatalf("%s New: %v", name, err)
		}
		if handler == nil {
			t.Fatalf("%s New returned nil handler", name)
		}
	}
}

// TestNewRejectsBadConfig proves an unknown limiter or an unparsable or invalid setting fails at New, not on a request.
func TestNewRejectsBadConfig(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	configs := map[string]*Config{
		"unknownLimiter": {Limiter: "leaky"},
		"badRate":        {Limiter: "tokenbucket", Rate: "fast", Burst: "3"},
		"zeroBurst":      {Limiter: "tokenbucket", Rate: "1", Burst: "0"},
		"badWindow":      {Limiter: "windowcounter", Limit: "3", Window: "minute"},
		"badSyncRate":    {Limiter: "windowcounter", Limit: "3", Window: "60s", SyncRate: "soon"},
	}
	for name, cfg := range configs {
		if _, err := New(context.Background(), next, cfg, "probe"); err == nil {
			t.Fatalf("%s: New returned nil error", name)
		}
	}
}

// TestCeilSeconds proves whole-second rounding up, zero for non-positive, and no overflow at the saturated duration.
func TestCeilSeconds(t *testing.T) {
	maxDuration := time.Duration(math.MaxInt64)
	cases := map[time.Duration]int64{
		-time.Second:     0,
		0:                0,
		time.Nanosecond:  1,
		time.Second:      1,
		time.Second + 1:  2,
		59 * time.Second: 59,
		maxDuration:      int64(maxDuration/time.Second) + 1,
	}
	for in, want := range cases {
		if got := ceilSeconds(in); got != want {
			t.Fatalf("ceilSeconds(%v) = %d, want %d", in, got, want)
		}
	}
}
