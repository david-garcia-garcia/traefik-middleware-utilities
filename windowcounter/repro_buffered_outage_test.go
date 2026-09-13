package windowcounter

import (
	"context"
	"testing"
	"time"
)

// TestRepro_BufferedTakeHidesRedisOutage proves buffered Take after Redis death returns nil error and denies after this node's limit.
func TestRepro_BufferedTakeHidesRedisOutage(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	const limit int64 = 2
	window := 2 * time.Hour

	allowed, estimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatalf("healthy take denied, estimated %v", estimated)
	}

	// Dest fail-closed only returns a Redis error after one missed sync_rate; freeze past that so this lock cannot pass on dest.
	fake.Kill()
	now = now.Add(time.Hour)
	limiter.SetNowForTest(func() time.Time { return now })

	takeUntilLocalDeny(t, limiter, "k", 1, limit, window)
	peekNilError(t, limiter, "k", limit, window)
}
