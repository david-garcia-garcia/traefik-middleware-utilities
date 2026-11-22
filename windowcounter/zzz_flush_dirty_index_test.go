package windowcounter

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"
)

// idleTickRatioFlushDirtyIndex is the compiled idle-flush ceiling: 200k live windows must stay below this times 10k.
const idleTickRatioFlushDirtyIndex = 3

// checkIndexesLockedFlushDirtyIndex asserts every live window sits in exactly one expiry bucket matching expireAt, and dirty
// equals the windows with localDelta > 0 or flushDelta > 0. Caller holds l.mu.
func checkIndexesLockedFlushDirtyIndex(t *testing.T, limiter *Limiter) {
	t.Helper()
	seenExpire := make(map[string]int64, len(limiter.windows))
	for expireAt, bucket := range limiter.expiry {
		for redisKey, state := range bucket {
			if state == nil {
				t.Fatalf("nil window in expiry bucket %d key %q", expireAt, redisKey)
			}
			if state.expireAt != expireAt {
				t.Fatalf("window %q in bucket %d, expireAt %d", redisKey, expireAt, state.expireAt)
			}
			if limiter.windows[redisKey] != state {
				t.Fatalf("expiry window %q missing from windows or pointer mismatch", redisKey)
			}
			if _, dup := seenExpire[redisKey]; dup {
				t.Fatalf("window %q in more than one expiry bucket", redisKey)
			}
			seenExpire[redisKey] = expireAt
		}
	}
	for redisKey, state := range limiter.windows {
		if _, ok := seenExpire[redisKey]; !ok {
			t.Fatalf("window %q missing from expiry", redisKey)
		}
		_, inDirty := limiter.dirty[redisKey]
		wantDirty := state.localDelta > 0 || state.flushDelta > 0
		if wantDirty != inDirty {
			t.Fatalf("window %q dirty=%v localDelta=%d flushDelta=%d", redisKey, inDirty, state.localDelta, state.flushDelta)
		}
	}
	for redisKey, state := range limiter.dirty {
		if limiter.windows[redisKey] != state {
			t.Fatalf("dirty window %q missing from windows or pointer mismatch", redisKey)
		}
		if state.localDelta == 0 && state.flushDelta == 0 {
			t.Fatalf("dirty window %q has no pending delta", redisKey)
		}
	}
}

// newLimiterFlushDirtyIndex builds a buffered limiter whose timer never fires, on a frozen clock.
func newLimiterFlushDirtyIndex(t *testing.T, addr string, now *time.Time) *Limiter {
	t.Helper()
	limiter, err := New(newSimpleRedisForTest(t, addr), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(limiter.Close)
	limiter.SetNowForTest(func() time.Time { return *now })
	return limiter
}

// seedIdleWindowsFlushDirtyIndex inserts count idle windows sharing expireAt. Caller must not hold l.mu.
func seedIdleWindowsFlushDirtyIndex(limiter *Limiter, count int, expireAt int64) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	for i := 0; i < count; i++ {
		limiter.putWindowLocked("idle:"+strconv.Itoa(i), &windowState{expireAt: expireAt})
	}
}

// medianDurationFlushDirtyIndex returns the median of samples.
func medianDurationFlushDirtyIndex(samples []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// TestFlushDirtyIndex_RandomizedIndexesAndCounts is Decision 5 (a): mixed Takes/Peeks, injected per-key flush failures, and
// clock advances keep the indexes aligned, and Redis matches Take counts after a clean flush.
func TestFlushDirtyIndex_RandomizedIndexesAndCounts(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	now := time.Unix(1_700_000_000, 0)
	limiter := newLimiterFlushDirtyIndex(t, addr, &now)
	ctx := context.Background()
	const limit int64 = 1 << 40
	keys := []string{"k0", "k1", "k2", "k3", "k4", "k5", "k6", "k7"}
	windows := []time.Duration{time.Second, 10 * time.Second, time.Minute}
	takes := map[string]int64{}
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // G404: deterministic test mix, not a secret

	assertIndexes := func() {
		t.Helper()
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		checkIndexesLockedFlushDirtyIndex(t, limiter)
	}
	assertIndexes()

	for step := 0; step < 200; step++ {
		switch rng.Intn(5) {
		case 0, 1:
			key := keys[rng.Intn(len(keys))]
			window := windows[rng.Intn(len(windows))]
			sliding, err := limiter.slidingAt(key, window)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := limiter.Take(ctx, key, limit, window); err != nil {
				t.Fatalf("step %d Take: %v", step, err)
			}
			takes[sliding.currentKey]++
		case 2:
			key := keys[rng.Intn(len(keys))]
			window := windows[rng.Intn(len(windows))]
			if _, err := limiter.Peek(ctx, key, limit, window); err != nil {
				t.Fatalf("step %d Peek: %v", step, err)
			}
		case 3:
			if rng.Intn(3) == 0 {
				fake.failNextFlushIncrBy(keys[rng.Intn(len(keys))]+":", 1)
			}
			_ = limiter.flushPending(ctx)
		default:
			now = now.Add(time.Duration(rng.Intn(30)+1) * time.Second)
		}
		assertIndexes()
	}

	fake.failNextFlushIncrBy("nomatch:", 0)
	for {
		limiter.mu.Lock()
		pending := len(limiter.dirty)
		limiter.mu.Unlock()
		if pending == 0 {
			break
		}
		if err := limiter.flushPending(ctx); err != nil {
			t.Fatal(err)
		}
		assertIndexes()
	}

	for redisKey, want := range takes {
		if got := fakeStoredCountForTest(t, fake, redisKey); got != want {
			t.Fatalf("Redis %s = %d, want %d Takes", redisKey, got, want)
		}
	}
}

// TestFlushDirtyIndex_IdleTickDoesNotScaleWithLiveWindows is Decision 5 (b): median idle flushPending at 200k live windows
// stays below 3× the median at 10k (an O(K) scan is ~20×).
func TestFlushDirtyIndex_IdleTickDoesNotScaleWithLiveWindows(t *testing.T) {
	at10k := nsPerIdleFlushFlushDirtyIndex(t, 10_000)
	at200k := nsPerIdleFlushFlushDirtyIndex(t, 200_000)
	if at200k > int64(idleTickRatioFlushDirtyIndex)*at10k {
		t.Fatalf("idle flush 200k=%d ns/op 10k=%d ns/op ratio=%.2f, want < %d",
			at200k, at10k, float64(at200k)/float64(at10k), idleTickRatioFlushDirtyIndex)
	}
	t.Logf("idle flush 10k=%d ns/op 200k=%d ns/op ratio=%.2f", at10k, at200k, float64(at200k)/float64(at10k))
}

// nsPerIdleFlushFlushDirtyIndex is ns/op of idle flushPending with liveWindows non-due idle windows.
func nsPerIdleFlushFlushDirtyIndex(t *testing.T, liveWindows int) int64 {
	t.Helper()
	_, addr := startTestFakeRedis(t)
	now := time.Unix(1_700_000_000, 0)
	limiter := newLimiterFlushDirtyIndex(t, addr, &now)
	seedIdleWindowsFlushDirtyIndex(limiter, liveWindows, now.Unix()+3600)
	limiter.mu.Lock()
	checkIndexesLockedFlushDirtyIndex(t, limiter)
	if got := len(limiter.windows); got != liveWindows {
		t.Fatalf("live windows %d, want %d", got, liveWindows)
	}
	if len(limiter.dirty) != 0 {
		t.Fatalf("dirty %d, want 0", len(limiter.dirty))
	}
	limiter.mu.Unlock()
	ctx := context.Background()
	if err := limiter.flushPending(ctx); err != nil {
		t.Fatal(err)
	}
	const rounds = 50_000
	const samples = 7
	times := make([]time.Duration, samples)
	for sample := 0; sample < samples; sample++ {
		start := time.Now()
		for i := 0; i < rounds; i++ {
			if err := limiter.flushPending(ctx); err != nil {
				t.Fatal(err)
			}
		}
		elapsed := time.Since(start)
		if elapsed <= 0 {
			t.Fatalf("idle flush at %d windows: %d rounds elapsed %s", liveWindows, rounds, elapsed)
		}
		times[sample] = elapsed / rounds
	}
	median := medianDurationFlushDirtyIndex(times)
	if median <= 0 {
		t.Fatalf("idle flush at %d windows median %s", liveWindows, median)
	}
	return int64(median)
}
