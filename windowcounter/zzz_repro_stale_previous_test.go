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

	status, takeErr := a.Take(context.Background(), "k", limit, window)
	if takeErr != nil {
		t.Fatal(takeErr)
	}
	if !status.Allowed {
		t.Fatalf("A window-1 take denied, estimated %v", status.Estimated)
	}
	status, takeErr = b.Take(context.Background(), "k", limit, window)
	if takeErr != nil {
		t.Fatal(takeErr)
	}
	if !status.Allowed {
		t.Fatalf("B window-1 take denied, estimated %v", status.Estimated)
	}

	// A flushes first so its redisKnown stays 1; B then writes the shared total of 2.
	a.Sleep()
	b.Sleep()

	now = start.Add(window)
	status, err = a.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("boundary Take allowed=%v estimated=%v (fresh previous would deny at estimated=3)", status.Allowed, status.Estimated)
	if status.Allowed {
		t.Fatalf("boundary take allowed=%v estimated=%v; want denied estimated=3 (current=1 + previous=2 * weight=1)", status.Allowed, status.Estimated)
	}
	if status.Estimated != 3 {
		t.Fatalf("boundary estimated %v, want 3", status.Estimated)
	}
}

// TestTake_BufferedUnflushedPreviousNotGet checks that a window roll with unflushed
// previous uses memory (redisKnown + localDelta) and does not GET that key.
func TestTake_BufferedUnflushedPreviousNotGet(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })

	window := 10 * time.Second
	const limit int64 = 3
	start := time.Unix(1_700_000_000, 0)
	now := start.Add(9 * time.Second)
	limiter.SetNowForTest(func() time.Time { return now })

	status, takeErr := limiter.Take(context.Background(), "k", limit, window)
	if takeErr != nil {
		t.Fatal(takeErr)
	}
	if !status.Allowed {
		t.Fatalf("window-1 take denied, estimated %v", status.Estimated)
	}
	getsAfterWindow1 := fake.getCallCount()

	now = start.Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	status, err = limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Allowed {
		t.Fatalf("window-2 take denied, estimated %v", status.Estimated)
	}
	if status.Estimated != 2 {
		t.Fatalf("unflushed previous estimated %v, want 2 (current=1 + previous localDelta=1)", status.Estimated)
	}
	if fake.getCallCount() != getsAfterWindow1+1 {
		t.Fatalf("GET count %d after roll, want %d (new current only)", fake.getCallCount(), getsAfterWindow1+1)
	}
}

// TestTake_BufferedPreviousRefreshBoundedBySyncRate checks that a flushed previous window is GET
// once when Take first uses it as previous, then at most once per sync_rate (not once per Take),
// and that another instance's late flush after the roll becomes visible after that interval.
func TestTake_BufferedPreviousRefreshBoundedBySyncRate(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	const syncRate = time.Hour
	a, err := New(newSimpleRedisForTest(t, addr), syncRate)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(newSimpleRedisForTest(t, addr), syncRate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.Close()
		b.Close()
	})

	ctx := context.Background()
	window := 4 * time.Hour
	windowSec := int64(window / time.Second)
	firstStart := int64(1_700_000_000) / windowSec * windowSec
	previousKey := redisWindowKey("k", firstStart)
	now := time.Unix(firstStart+60, 0)
	clock := func() time.Time { return now }
	a.SetNowForTest(clock)
	b.SetNowForTest(clock)
	const limit int64 = 1000

	for _, limiter := range []*Limiter{a, b} {
		if _, takeErr := limiter.Take(ctx, "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	if flushErr := a.flushPending(ctx); flushErr != nil {
		t.Fatal(flushErr)
	}

	// Next window: B has not flushed yet. A's previous is GET once, not on every Take.
	now = time.Unix(firstStart+windowSec+60, 0)
	getsBefore := fake.getCallCount()
	for i := 0; i < 20; i++ {
		if _, takeErr := a.Take(ctx, "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	if got := fake.getCallCount() - getsBefore; got != 2 {
		t.Fatalf("GETs for 20 Takes after roll = %d, want 2 (current first sight + one previous refresh)", got)
	}
	if got := redisKnownForTest(a, previousKey); got != 1 {
		t.Fatalf("previous redis_known %d, want 1 before B flushes", got)
	}

	// B flushes late, after the roll. Within sync_rate A keeps memory and does not GET.
	if flushErr := b.flushPending(ctx); flushErr != nil {
		t.Fatal(flushErr)
	}
	getsBefore = fake.getCallCount()
	if _, takeErr := a.Take(ctx, "k", limit, window); takeErr != nil {
		t.Fatal(takeErr)
	}
	if got := fake.getCallCount() - getsBefore; got != 0 {
		t.Fatalf("GETs within sync_rate of the last previous refresh = %d, want 0", got)
	}

	// One sync_rate later A refreshes previous once and sees B's late flush.
	now = now.Add(syncRate)
	if _, takeErr := a.Take(ctx, "k", limit, window); takeErr != nil {
		t.Fatal(takeErr)
	}
	if got := fake.getCallCount() - getsBefore; got != 1 {
		t.Fatalf("GETs after sync_rate = %d, want 1 (previous refresh)", got)
	}
	if got := redisKnownForTest(a, previousKey); got != 2 {
		t.Fatalf("previous redis_known %d after refresh, want 2 (A + B)", got)
	}
}

// redisKnownForTest reads redis_known for one buffered window key under the limiter mutex.
func redisKnownForTest(limiter *Limiter, redisKey string) int64 {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	state := limiter.windows[redisKey]
	if state == nil {
		return -1
	}
	return state.redisKnown
}
