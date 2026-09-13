package tokenbucket

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestTTLTruncation_RejectsFractionalTTL proves NewMemory and NewRedis reject a
// ttl Redis cannot pass as EXPIRE seconds. Whole-second ttl still constructs.
func TestTTLTruncation_RejectsFractionalTTL(t *testing.T) {
	fractional := 1500 * time.Millisecond
	if _, err := NewMemory(1, 2, time.Microsecond, fractional); !errors.Is(err, errTTL) {
		t.Fatalf("NewMemory 1500ms: %v", err)
	}
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	if _, err := NewRedis(client, 1, 2, time.Microsecond, fractional); !errors.Is(err, errTTL) {
		t.Fatalf("NewRedis 1500ms: %v", err)
	}
	if eval := fake.lastEvalCommand(); len(eval) != 0 {
		t.Fatalf("NewRedis 1500ms sent EVAL %v", eval)
	}

	whole := 2 * time.Second
	if _, err := NewMemory(1, 2, time.Microsecond, whole); err != nil {
		t.Fatalf("NewMemory 2s: %v", err)
	}
	if _, err := NewRedis(client, 1, 2, time.Microsecond, whole); err != nil {
		t.Fatalf("NewRedis 2s: %v", err)
	}

	t0 := time.Unix(1_700_000_000, 0)
	limiter, err := NewRedis(client, 1, 2, time.Microsecond, whole)
	if err != nil {
		t.Fatal(err)
	}
	limiter.SetNowForTest(func() time.Time { return t0 })
	if _, _, allowErr := limiter.Allow(context.Background(), "ttl-whole"); allowErr != nil {
		t.Fatal(allowErr)
	}
	eval := fake.lastEvalCommand()
	if len(eval) < 9 || eval[0] != "EVAL" {
		t.Fatalf("eval argv %v", eval)
	}
	if eval[6] != "2" {
		t.Fatalf("Redis ARGV ttl = %q, want 2; argv %v", eval[6], eval)
	}
}
