package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// TestAllow_MaxDelayZeroWaitDurationTruncFailOpen: maxDelay=0 and a high rate
// make waitMicro a fraction of a microsecond. consumeOne refunds (waitMicro > 0)
// but waitDuration truncates to 0, so allowedFromWait(0, 0) admits the refunded
// consume — fail-open, and the next Allow can repeat forever.
func TestAllow_MaxDelayZeroWaitDurationTruncFailOpen(t *testing.T) {
	const rate = 1e12
	limiter, err := NewMemory(rate, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	first, _, firstErr := limiter.Allow(context.Background(), "k")
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if !first {
		t.Fatal("first Allow must spend the burst token")
	}

	second, _, secondErr := limiter.Allow(context.Background(), "k")
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	if second {
		t.Fatalf("second Allow admitted with maxDelay=0: waitDuration truncated below 1ns while consumeOne refunds (rate=%g waitMicro=%g)",
			rate, 1e6/rate)
	}

	third, _, thirdErr := limiter.Allow(context.Background(), "k")
	if thirdErr != nil {
		t.Fatal(thirdErr)
	}
	if third {
		t.Fatal("third Allow admitted; refunded consume left the bucket reusable")
	}
}
