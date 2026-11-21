package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// TestAllow_MaxDelayTruncationFailOpen checks whether consume refunds in whole
// microseconds while Allow admits using the original Duration, so a refunded
// consume can still return allowed=true (fail-open / infinite bypass).
func TestAllow_MaxDelayTruncationFailOpen(t *testing.T) {
	t.Run("maxDelay_1500ns_rate_for_1.2us_wait", func(t *testing.T) {
		reproMaxDelayTruncationFailOpen(t, 1e6/1.2, 1500*time.Nanosecond)
	})
	t.Run("maxDelay_1us_rate_999999", func(t *testing.T) {
		reproMaxDelayTruncationFailOpen(t, 999999, time.Microsecond)
	})
}

// waitDurationForTest converts wait microseconds the same way dest waitDuration did,
// so the dest split (refund vs Duration admit) stays visible after that helper is deleted.
func waitDurationForTest(waitMicro float64) time.Duration {
	if waitMicro <= 0 {
		return 0
	}
	return time.Duration(waitMicro * float64(time.Microsecond))
}

// reproMaxDelayTruncationFailOpen freezes the clock, spends the burst token, then
// issues two more Allow calls at the same instant. Fail-open is second and third
// both admitted while consumeOne refunded (waitMicro > maxDelay.Microseconds()).
func reproMaxDelayTruncationFailOpen(t *testing.T, rate float64, maxDelay time.Duration) {
	t.Helper()
	limiter, err := NewMemory(rate, 1, maxDelay, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	maxDelayMicro := maxDelay.Microseconds()
	limitPerMicro := rate / 1e6
	nowMicro := now.UnixMicro()
	_, _, waitMicro := consumeOne(0, nowMicro, limitPerMicro, 1, nowMicro, maxDelayMicro)
	wait := waitDurationForTest(waitMicro)
	refunds := waitMicro > float64(maxDelayMicro)
	admits := wait <= maxDelay
	t.Logf("waitMicro=%v maxDelayMicro=%d wait=%s maxDelay=%s refunds=%v allowedFromWait=%v",
		waitMicro, maxDelayMicro, wait, maxDelay, refunds, admits)

	first, _, firstErr := limiter.Allow(context.Background(), "k")
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if !first {
		t.Fatal("first Allow must consume the burst token")
	}

	second, _, secondErr := limiter.Allow(context.Background(), "k")
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	tokensAfterSecond := limiter.buckets["k"].tokens

	third, _, thirdErr := limiter.Allow(context.Background(), "k")
	if thirdErr != nil {
		t.Fatal(thirdErr)
	}

	t.Logf("first=%v second=%v tokensAfterSecond=%v third=%v", first, second, tokensAfterSecond, third)

	if refunds && admits && second && tokensAfterSecond >= 0 && third {
		t.Fatalf("fail-open: refunded consume still admitted (second=%v tokens=%v third=%v)", second, tokensAfterSecond, third)
	}
}

// TestRedis_MaxDelayTruncationFailOpen is the Redis Allow mapping of the 1500ns
// dest split: Lua refunds in whole microseconds; dest Duration admit would still allow.
func TestRedis_MaxDelayTruncationFailOpen(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	const rate = 1e6 / 1.2
	maxDelay := 1500 * time.Nanosecond
	limiter, err := NewRedis(client, rate, 1, maxDelay, 2*time.Second)
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
		t.Fatal("first Allow must consume the burst token")
	}

	second, _, secondErr := limiter.Allow(context.Background(), "k")
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	if second {
		t.Fatal("fail-open: Redis Allow admitted a refunded consume")
	}

	third, _, thirdErr := limiter.Allow(context.Background(), "k")
	if thirdErr != nil {
		t.Fatal(thirdErr)
	}
	if third {
		t.Fatal("fail-open: following Redis Allow was a stacked consume")
	}
}
