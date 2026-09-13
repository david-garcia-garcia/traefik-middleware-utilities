package tokenbucket

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestRepro_NaNRateRejected asserts NewMemory and NewRedis reject a NaN rate.
func TestRepro_NaNRateRejected(t *testing.T) {
	t.Run("NewMemory", func(t *testing.T) {
		limiter, err := NewMemory(math.NaN(), 1, 0, 2*time.Second)
		if err == nil {
			t.Errorf("NewMemory(NaN) returned limiter %v, want error", limiter)
			return
		}
		if !errors.Is(err, errRate) {
			t.Fatalf("NewMemory(NaN) err %v, want errRate", err)
		}
		if limiter != nil {
			t.Fatalf("NewMemory(NaN) limiter %v with err %v", limiter, err)
		}
	})
	t.Run("NewRedis", func(t *testing.T) {
		client := simpleredis.New(simpleredis.Config{Host: "127.0.0.1:1"})
		limiter, err := NewRedis(client, math.NaN(), 1, 0, 2*time.Second)
		if err == nil {
			t.Errorf("NewRedis(NaN) returned limiter %v, want error", limiter)
			return
		}
		if !errors.Is(err, errRate) {
			t.Fatalf("NewRedis(NaN) err %v, want errRate", err)
		}
		if limiter != nil {
			t.Fatalf("NewRedis(NaN) limiter %v with err %v", limiter, err)
		}
	})
}

// TestRepro_NaNRateAllowFailOpen records Allow when NewMemory accepts a NaN rate.
// After New rejects NaN this path is unreachable and the test skips.
func TestRepro_NaNRateAllowFailOpen(t *testing.T) {
	limiter, err := NewMemory(math.NaN(), 1, 0, 2*time.Second)
	if err != nil {
		t.Skipf("NewMemory rejected NaN (%v); Allow fail-open not reachable", err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	alwaysAllowed := true
	for i := 0; i < 5; i++ {
		allowed, wait, allowErr := limiter.Allow(context.Background(), "k")
		t.Logf("Allow[%d] allowed=%v wait=%v err=%v", i, allowed, wait, allowErr)
		if allowErr != nil {
			t.Errorf("Allow[%d] err %v", i, allowErr)
			alwaysAllowed = false
			continue
		}
		if !allowed {
			alwaysAllowed = false
		}
	}
	if alwaysAllowed {
		t.Fatal("NaN rate Allow always returned true (fail-open)")
	}
}
