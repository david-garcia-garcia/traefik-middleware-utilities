package tokenbucket

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRepro_StaleNowRewindsLastDoubleRefill: consumeOne persists nowMicro as last
// even when nowMicro < last. A later sample then refills an interval already
// applied. Memory samples now() outside the mutex, so a stale timestamp can
// overwrite a newer last.
func TestRepro_StaleNowRewindsLastDoubleRefill(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(5 * time.Second)
	t2 := t0.Add(10 * time.Second)

	t.Run("sequential", func(t *testing.T) {
		limiter := newBurst1Delay0(t)
		limiter.SetNowForTest(func() time.Time { return t0 })
		mustAllow(t, limiter, "k", true, "init at t0")

		limiter.SetNowForTest(func() time.Time { return t2 })
		mustAllow(t, limiter, "k", true, "refill at t2")

		limiter.SetNowForTest(func() time.Time { return t1 })
		mustAllow(t, limiter, "k", false, "stale t1 must deny")

		limiter.SetNowForTest(func() time.Time { return t2 })
		allowed, _, err := limiter.Allow(context.Background(), "k")
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatalf("Allow at t2 after stale t1 admitted; last rewound to t1 and refilled %s already granted at t2 (last=%d want last at t2)",
				t2.Sub(t1), limiter.buckets["k"].last)
		}
	})

	t.Run("goroutines_stale_samples_before_fresh_lock", func(t *testing.T) {
		limiter := newBurst1Delay0(t)
		limiter.SetNowForTest(func() time.Time { return t0 })
		mustAllow(t, limiter, "k", true, "init at t0")

		staleSampled := make(chan struct{})
		freshFinished := make(chan struct{})
		var nowCalls atomic.Int64
		limiter.SetNowForTest(func() time.Time {
			if nowCalls.Add(1) == 1 {
				close(staleSampled)
				<-freshFinished
				return t1
			}
			return t2
		})

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, _ = limiter.Allow(context.Background(), "k")
		}()
		<-staleSampled
		go func() {
			defer wg.Done()
			_, _, _ = limiter.Allow(context.Background(), "k")
			close(freshFinished)
		}()
		wg.Wait()

		limiter.SetNowForTest(func() time.Time { return t2 })
		allowed, _, err := limiter.Allow(context.Background(), "k")
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatalf("after stale now (t1) overwrote fresh last (t2), Allow at t2 admitted; last=%d", limiter.buckets["k"].last)
		}
	})

	t.Run("lua_hset_last_is_persist_max", func(t *testing.T) {
		if strings.Contains(allowScript, "'last', t") {
			t.Fatal("allowScript HSET last is raw t; want persist-max of bucket.last and t")
		}
		if !strings.Contains(allowScript, "if t < last then") {
			t.Fatal("allowScript dropped the elapsed clamp")
		}
	})
}

func newBurst1Delay0(t *testing.T) *Memory {
	t.Helper()
	limiter, err := NewMemory(1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}

func mustAllow(t *testing.T, limiter *Memory, key string, want bool, step string) {
	t.Helper()
	got, _, err := limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("%s: %v", step, err)
	}
	if got != want {
		t.Fatalf("%s: allowed=%v want %v", step, got, want)
	}
}
