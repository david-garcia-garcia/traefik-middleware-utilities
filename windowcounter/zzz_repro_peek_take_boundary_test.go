package windowcounter

import (
	"context"
	"testing"
	"time"
)

// TestRepro_PeekAllowsWhenNextTakeDenies locks occupancy: after exactly limit Takes,
// Peek allows with estimated equal to limit, and the next Take denies with estimated limit+1.
func TestRepro_PeekAllowsWhenNextTakeDenies(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		reproPeekAllowsWhenNextTakeDenies(t, 0)
	})
	t.Run("buffered", func(t *testing.T) {
		reproPeekAllowsWhenNextTakeDenies(t, time.Hour)
	})
}

// reproPeekAllowsWhenNextTakeDenies Takes exactly limit times, then checks Peek occupancy versus the next Take.
func reproPeekAllowsWhenNextTakeDenies(t *testing.T, syncRate time.Duration) {
	t.Helper()
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, syncRate)
	if err != nil {
		t.Fatal(err)
	}
	if syncRate > 0 {
		t.Cleanup(func() { limiter.Close() })
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 3
	window := time.Minute
	for i := int64(0); i < limit; i++ {
		status, takeErr := limiter.Take(context.Background(), "k", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !status.Allowed {
			t.Fatalf("fill take %d denied, estimated %v", i+1, status.Estimated)
		}
	}
	peekStatus, err := limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	takeStatus, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !peekStatus.Allowed {
		t.Fatalf("Peek after %d takes denied, estimated %v", limit, peekStatus.Estimated)
	}
	if peekStatus.Estimated != float64(limit) {
		t.Fatalf("Peek estimated %v, want %d", peekStatus.Estimated, limit)
	}
	if takeStatus.Allowed {
		t.Fatalf("Take after occupancy %d allowed, estimated %v", limit, takeStatus.Estimated)
	}
	if takeStatus.Estimated != float64(limit)+1 {
		t.Fatalf("Take estimated %v, want %d", takeStatus.Estimated, limit+1)
	}
}
