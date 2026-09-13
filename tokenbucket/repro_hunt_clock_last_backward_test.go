package tokenbucket

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestRepro_StaleNowRewindsLastDoubleRefill proves persisted last does not
// rewind: sequential backward now keeps last at t2, overlapping Allows cannot
// sample now before mu, and Lua HSET last is persist-max not raw t.
func TestRepro_StaleNowRewindsLastDoubleRefill(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(5 * time.Second)
	t2 := t0.Add(10 * time.Second)

	t.Run("sequential", func(t *testing.T) {
		limiter := newBurst1Delay0(t)
		limiter.SetNowForTest(func() time.Time { return t0 })
		mustAllow(t, limiter, true, "init at t0")

		limiter.SetNowForTest(func() time.Time { return t2 })
		mustAllow(t, limiter, true, "refill at t2")

		limiter.SetNowForTest(func() time.Time { return t1 })
		mustAllow(t, limiter, false, "stale t1 must deny")

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
		mustAllow(t, limiter, true, "init at t0")

		firstInNow := make(chan struct{})
		proceed := make(chan struct{})
		var nowCalls atomic.Int64
		limiter.SetNowForTest(func() time.Time {
			if nowCalls.Add(1) == 1 {
				close(firstInNow)
				<-proceed
				return t1
			}
			return t2
		})

		firstDone := make(chan struct{})
		go func() {
			defer close(firstDone)
			_, _, _ = limiter.Allow(context.Background(), "k")
		}()
		<-firstInNow
		secondDone := make(chan struct{})
		go func() {
			defer close(secondDone)
			_, _, _ = limiter.Allow(context.Background(), "k")
		}()
		select {
		case <-secondDone:
			close(proceed)
			<-firstDone
			t.Fatal("second Allow finished while first now() was waiting; now() sampled before mu")
		case <-time.After(500 * time.Millisecond):
			// Second Allow is blocked: first now() still holds mu.
		}
		close(proceed)
		<-firstDone
		<-secondDone

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
	limiter, err := NewMemory(1, 1, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}

func mustAllow(t *testing.T, limiter *Memory, want bool, step string) {
	t.Helper()
	got, _, err := limiter.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("%s: %v", step, err)
	}
	if got != want {
		t.Fatalf("%s: allowed=%v want %v", step, got, want)
	}
}
