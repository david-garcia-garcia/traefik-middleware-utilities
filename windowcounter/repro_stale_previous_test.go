package windowcounter

import (
	"context"
	"testing"
	"time"
)

// TestRepro_BufferedStalePreviousAtBoundary checks whether client A, after flushing
// first in window 1, admits a sliding-boundary hit in window 2 from a stale previous
// count while Redis already holds both clients' hits.
func TestRepro_BufferedStalePreviousAtBoundary(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := newSimpleRedisForTest(t, addr)
	bClient := newSimpleRedisForTest(t, addr)
	a, err := New(aClient, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(bClient, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.Close()
		b.Close()
	})

	window := 10 * time.Second
	const limit int64 = 2
	start := time.Unix(1_700_000_000, 0)
	now := start.Add(9 * time.Second)
	clock := func() time.Time { return now }
	a.SetNowForTest(clock)
	b.SetNowForTest(clock)

	allowed, estimated, takeErr := a.Take(context.Background(), "k", limit, window)
	if takeErr != nil {
		t.Fatal(takeErr)
	}
	if !allowed {
		t.Fatalf("A window-1 take denied, estimated %v", estimated)
	}
	allowed, estimated, takeErr = b.Take(context.Background(), "k", limit, window)
	if takeErr != nil {
		t.Fatal(takeErr)
	}
	if !allowed {
		t.Fatalf("B window-1 take denied, estimated %v", estimated)
	}

	// A flushes first so its redisKnown stays 1; B then writes the shared total of 2.
	a.Sleep()
	b.Sleep()

	now = start.Add(window)
	allowed, estimated, err = a.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("boundary Take allowed=%v estimated=%v (fresh previous would deny at estimated=3)", allowed, estimated)
	if allowed {
		t.Fatalf("boundary take allowed=%v estimated=%v; want denied estimated=3 (current=1 + previous=2 * weight=1)", allowed, estimated)
	}
	if estimated != 3 {
		t.Fatalf("boundary estimated %v, want 3", estimated)
	}
}
