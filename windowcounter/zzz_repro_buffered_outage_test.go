package windowcounter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
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

	status, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Allowed {
		t.Fatalf("healthy take denied, estimated %v", status.Estimated)
	}

	// Dest fail-closed only returns a Redis error after one missed sync_rate; freeze past that so this lock cannot pass on dest.
	fake.Kill()
	now = now.Add(time.Hour)
	limiter.SetNowForTest(func() time.Time { return now })

	takeUntilLocalDeny(t, limiter, "k", 1, limit, window)
	peekNilError(t, limiter, "k", limit, window)
}

// outageTestClock aligns a clock to the start of a window plus one minute.
func outageTestClock(window time.Duration) time.Time {
	windowSec := int64(window / time.Second)
	return time.Unix(int64(1_700_000_000)/windowSec*windowSec+60, 0)
}

// rememberOutageForTest stores a Redis failure as if a GET or flush had just failed.
func rememberOutageForTest(limiter *Limiter) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	limiter.rememberOutageLocked(errors.New(simpleredis.RedisUnreachable))
}

// outageStoredForTest reports whether the limiter still holds an outage error.
func outageStoredForTest(limiter *Limiter) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.lastOutageErr != nil
}

// TestBuffered_PeekOnlyRecoversAfterTransientOutage: a Peek-only limiter (no pending delta, so no
// flush to clear the outage) must not stay blind after one failed GET. Within sync_rate it keeps
// its own view without GET; after sync_rate it GETs again, sees the shared count, and clears the outage.
func TestBuffered_PeekOnlyRecoversAfterTransientOutage(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	const syncRate = time.Hour
	peeker, err := New(newSimpleRedisForTest(t, addr), syncRate)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := New(newSimpleRedisForTest(t, addr), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		peeker.Close()
		writer.Close()
	})
	window := 4 * time.Hour
	now := outageTestClock(window)
	clock := func() time.Time { return now }
	peeker.SetNowForTest(clock)
	writer.SetNowForTest(clock)
	ctx := context.Background()
	const limit int64 = 5

	for i := 0; i < 10; i++ {
		if _, takeErr := writer.Take(ctx, "hot", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	rememberOutageForTest(peeker)

	getsBefore := fake.getCallCount()
	status, err := peeker.Peek(ctx, "hot", limit, window)
	if err != nil {
		t.Fatalf("Peek during stored outage err %v, want nil", err)
	}
	if !status.Allowed || status.Estimated != 0 {
		t.Fatalf("Peek during stored outage allowed=%v estimated=%v, want local view allowed=true estimated=0", status.Allowed, status.Estimated)
	}
	if got := fake.getCallCount() - getsBefore; got != 0 {
		t.Fatalf("Peek GETs within sync_rate of the outage = %d, want 0", got)
	}

	now = now.Add(syncRate)
	status, err = peeker.Peek(ctx, "hot", limit, window)
	if err != nil {
		t.Fatalf("Peek after sync_rate err %v, want nil", err)
	}
	if status.Allowed || status.Estimated != 10 {
		t.Fatalf("Peek after sync_rate allowed=%v estimated=%v, want denied at the shared count 10", status.Allowed, status.Estimated)
	}
	if outageStoredForTest(peeker) {
		t.Fatal("outage still stored after a successful GET")
	}
}

// TestBuffered_TakeOutageSeedGetsSharedCountFromFlush: a Take during a stored outage seeds the key at
// zero and admits on this instance's count. The next successful flush returns the shared count,
// clears the outage, and later Takes estimate from Redis again.
func TestBuffered_TakeOutageSeedGetsSharedCountFromFlush(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	const syncRate = time.Hour
	taker, err := New(newSimpleRedisForTest(t, addr), syncRate)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := New(newSimpleRedisForTest(t, addr), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		taker.Close()
		writer.Close()
	})
	window := 4 * time.Hour
	now := outageTestClock(window)
	clock := func() time.Time { return now }
	taker.SetNowForTest(clock)
	writer.SetNowForTest(clock)
	ctx := context.Background()
	const limit int64 = 100

	for i := 0; i < 3; i++ {
		if _, takeErr := writer.Take(ctx, "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	rememberOutageForTest(taker)

	status, err := taker.Take(ctx, "k", limit, window)
	if err != nil {
		t.Fatalf("Take during stored outage err %v, want nil", err)
	}
	if status.Estimated != 1 {
		t.Fatalf("Take during stored outage estimated %v, want 1 (this instance only)", status.Estimated)
	}

	if flushErr := taker.flushPending(ctx); flushErr != nil {
		t.Fatal(flushErr)
	}
	if outageStoredForTest(taker) {
		t.Fatal("outage still stored after a successful flush")
	}
	status, err = taker.Take(ctx, "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if status.Estimated != 5 {
		t.Fatalf("Take after recovery estimated %v, want 5 (3 shared + 1 flushed + 1 new)", status.Estimated)
	}
}
