package tokenbucket

import (
	"context"
	"errors"
	"testing"
	"time"
)

// countAdmits14173 calls allow up to attempts times and returns how many were admitted before the first deny.
func countAdmits14173(t *testing.T, attempts int, allow func() (bool, error)) int {
	t.Helper()
	admitted := 0
	for i := 0; i < attempts; i++ {
		ok, err := allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i+1, err)
		}
		if !ok {
			return admitted
		}
		admitted++
	}
	return admitted
}

// TestTraefik14173_MemoryCanceledContext proves a pre-canceled ctx is context.Canceled with a zero Status and does not
// charge the source, so the next Allow still gets the full burst.
func TestTraefik14173_MemoryCanceledContext(t *testing.T) {
	const burst = int64(3)
	limiter, err := NewMemory(1.0/60, burst, 0, 120*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	status, err := limiter.Allow(canceled, "client-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request returned err %v, want context.Canceled", err)
	}
	if status.Allowed {
		t.Fatal("canceled request reported allowed")
	}
	if status != (Status{}) {
		t.Fatalf("canceled request returned %+v, want the zero Status", status)
	}
	got := countAdmits14173(t, int(burst)+5, func() (bool, error) {
		next, allowErr := limiter.Allow(context.Background(), "client-1")
		return next.Allowed, allowErr
	})
	if got != int(burst) {
		t.Fatalf("after a canceled request the source got %d admits, want the full burst %d", got, burst)
	}
}
