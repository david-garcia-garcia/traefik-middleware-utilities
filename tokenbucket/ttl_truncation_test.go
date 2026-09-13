package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// TestTTLTruncation_MemoryVsRedis proves Memory expireAt uses the full ttl
// Duration while Redis EVAL ARGV passes ttlSeconds (integer seconds). Spec
// requires the same Allow sequence on both stores. Fake Redis does not EXPIRE,
// so Redis expiry is observed via lastEvalCommand ARGV, not by waiting for the
// hash to drop. Dest FAIL: New accepts 1500ms; ARGV ttl is 1; Memory is not a
// fresh burst at +1200ms.
func TestTTLTruncation_MemoryVsRedis(t *testing.T) {
	ttl := 1500 * time.Millisecond
	burst := int64(2)
	rate := float64(1)
	maxDelay := time.Microsecond
	key := "ttl-truncation"

	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	redisLimiter, err := NewRedis(client, rate, burst, maxDelay, ttl)
	if err != nil {
		t.Fatalf("NewRedis rejected ttl=%s: %v", ttl, err)
	}

	t0 := time.Unix(1_700_000_000, 0)
	redisLimiter.SetNowForTest(func() time.Time { return t0 })
	if _, _, allowErr := redisLimiter.Allow(context.Background(), key); allowErr != nil {
		t.Fatal(allowErr)
	}

	eval := fake.lastEvalCommand()
	if len(eval) < 9 || eval[0] != "EVAL" {
		t.Fatalf("eval argv %v", eval)
	}
	// EVAL script numkeys key limit burst TTL now maxdelay
	redisTTLArg := eval[6]

	admitsAt1200 := memoryBurstAdmitsAfter(t, rate, burst, maxDelay, ttl, key, t0, 1200*time.Millisecond)
	admitsAt1500 := memoryBurstAdmitsAfter(t, rate, burst, maxDelay, ttl, key, t0, 1500*time.Millisecond)

	if redisTTLArg != "1" {
		t.Fatalf("Redis ARGV ttl = %q, want 1 from ttlSeconds(1500ms); argv %v", redisTTLArg, eval)
	}
	if admitsAt1500 != int(burst) {
		t.Fatalf("Memory at +1500ms admitted %d, want fresh burst %d", admitsAt1500, burst)
	}
	// Redis EXPIRE 1 would drop the key before +1200ms, so agreement means a fresh burst.
	if admitsAt1200 != int(burst) {
		t.Fatalf("Memory at +1200ms admitted %d want %d (fresh burst to match Redis EXPIRE 1); Redis ARGV ttl=%q argv=%v", admitsAt1200, burst, redisTTLArg, eval)
	}
}

// memoryBurstAdmitsAfter depletes burst at t0, advances idle, then counts consecutive admits.
func memoryBurstAdmitsAfter(t *testing.T, rate float64, burst int64, maxDelay, ttl time.Duration, key string, t0 time.Time, idle time.Duration) int {
	t.Helper()
	limiter, err := NewMemory(rate, burst, maxDelay, ttl)
	if err != nil {
		t.Fatalf("NewMemory rejected ttl=%s: %v", ttl, err)
	}
	now := t0
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < int(burst); i++ {
		allowed, _, allowErr := limiter.Allow(context.Background(), key)
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed {
			t.Fatalf("deplete burst %d denied", i+1)
		}
	}
	now = t0.Add(idle)
	limiter.SetNowForTest(func() time.Time { return now })
	admits := 0
	for i := 0; i < int(burst); i++ {
		allowed, _, allowErr := limiter.Allow(context.Background(), key)
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if allowed {
			admits++
		}
	}
	return admits
}
