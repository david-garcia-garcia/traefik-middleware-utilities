package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// TestAllow_WaitDurationOverflowFailOpen: limitPerMicro is tiny but not 0.
// After a seeded empty bucket (last=now, not last=0), one consume makes
// waitMicro so large that waitDuration overflows to a negative Duration and
// allowedFromWait fail-opens while consumeOne refunds.
func TestAllow_WaitDurationOverflowFailOpen(t *testing.T) {
	const rate = 1e-12
	limiter, err := NewMemory(rate, 1, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	limiter.buckets["k"] = &memEntry{
		tokens:   0,
		last:     now.UnixMicro(),
		expireAt: now.Add(testTTL),
	}

	allowed, _, allowErr := limiter.Allow(context.Background(), "k")
	if allowErr != nil {
		t.Fatal(allowErr)
	}
	waitMicro := 1 / (rate / 1e6)
	wait := waitDurationForTest(waitMicro)
	if allowed {
		t.Fatalf("Allow admitted empty bucket with waitDuration=%s waitMicro=%g (overflow fail-open vs hour maxDelay)",
			wait, waitMicro)
	}
}
