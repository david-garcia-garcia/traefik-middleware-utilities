package windowcounter

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// flushTestWindow is the window every flush batch test Takes on.
const flushTestWindow = time.Minute

// newFlushTestLimiter builds a buffered limiter whose timer never fires (sync_rate one hour) on a frozen clock.
func newFlushTestLimiter(t *testing.T, addr string) *Limiter {
	t.Helper()
	limiter, err := New(newSimpleRedisForTest(t, addr), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(limiter.Close)
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	return limiter
}

// takeFlushTestHit Takes one hit on key and fails on a non-nil error.
func takeFlushTestHit(t *testing.T, limiter *Limiter, key string) {
	t.Helper()
	if _, err := limiter.Take(context.Background(), key, 1000, flushTestWindow); err != nil {
		t.Fatalf("Take(%q): %v", key, err)
	}
}

// currentWindowKeyForTest is the Redis key Take uses for key's current window at the limiter clock.
func currentWindowKeyForTest(t *testing.T, limiter *Limiter, key string) string {
	t.Helper()
	sliding, err := limiter.slidingAt(key, flushTestWindow)
	if err != nil {
		t.Fatal(err)
	}
	return sliding.currentKey
}

// windowStateForTest copies one buffered window under the limiter mutex; a missing window fails the test.
func windowStateForTest(t *testing.T, limiter *Limiter, redisKey string) windowState {
	t.Helper()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	state := limiter.windows[redisKey]
	if state == nil {
		t.Fatalf("no buffered window %q", redisKey)
	}
	return *state
}

// fakeStoredCountForTest is the fake's integer value for redisKey, 0 when missing.
func fakeStoredCountForTest(t *testing.T, fake *testFakeRedis, redisKey string) int64 {
	t.Helper()
	fake.mu.Lock()
	raw, found := fake.store[redisKey]
	fake.mu.Unlock()
	if !found {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatalf("fake %q holds %q: %v", redisKey, raw, err)
	}
	return n
}

// storedOutageErrForTest returns the flush or GET error the limiter holds.
func storedOutageErrForTest(limiter *Limiter) error {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.lastOutageErr
}

// TestFlush_BatchOneKeyFailsOthersApply: inside one EVAL, a key whose INCRBY fails keeps its delta while the other key
// applies; the next flush sends only new deltas, so the applied key is counted once.
func TestFlush_BatchOneKeyFailsOthersApply(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	ctx := context.Background()
	takeFlushTestHit(t, limiter, "a")
	takeFlushTestHit(t, limiter, "b")
	aKey := currentWindowKeyForTest(t, limiter, "a")
	bKey := currentWindowKeyForTest(t, limiter, "b")
	fake.failNextFlushIncrBy("b:", 1)

	flushErr := limiter.flushPending(ctx)
	if flushErr == nil || !strings.Contains(flushErr.Error(), "injected incrby failure") {
		t.Fatalf("flush error %v, want b's INCRBY failure", flushErr)
	}
	if keys := fake.lastFlushBatchKeys(); len(keys) != 2 {
		t.Fatalf("flush EVAL KEYS %v, want a and b in one EVAL", keys)
	}
	applied := windowStateForTest(t, limiter, aKey)
	if applied.redisKnown != 1 || applied.localDelta != 0 || applied.flushDelta != 0 {
		t.Fatalf("a after flush %+v, want redis_known 1, local_delta 0, not in flight", applied)
	}
	failed := windowStateForTest(t, limiter, bKey)
	if failed.localDelta != 1 || failed.flushDelta != 0 {
		t.Fatalf("b after flush %+v, want local_delta 1 kept, not in flight", failed)
	}
	if stored := storedOutageErrForTest(limiter); stored == nil || !strings.Contains(stored.Error(), "injected incrby failure") {
		t.Fatalf("stored error %v, want b's failure", stored)
	}
	// A Take after the failed flush keeps the local limit with a nil error.
	takeFlushTestHit(t, limiter, "a")

	if err := limiter.flushPending(ctx); err != nil {
		t.Fatal(err)
	}
	if got := fakeStoredCountForTest(t, fake, aKey); got != 2 {
		t.Fatalf("Redis a = %d, want 2 (first delta once plus the later hit)", got)
	}
	if got := fakeStoredCountForTest(t, fake, bKey); got != 1 {
		t.Fatalf("Redis b = %d, want 1 (retried delta once)", got)
	}
}

// TestFlush_TakeDuringBatchKeepsHit: hits counted while a batch EVAL is in flight stay in local_delta after it applies.
func TestFlush_TakeDuringBatchKeepsHit(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	for i := 0; i < 3; i++ {
		takeFlushTestHit(t, limiter, "k")
	}
	redisKey := currentWindowKeyForTest(t, limiter, "k")
	entered, release := fake.holdNextFlush()
	defer release()

	flushDone := make(chan error, 1)
	go func() { flushDone <- limiter.flushPending(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("flush EVAL never reached the fake")
	}
	// Two hits while the EVAL carrying 3 is held.
	takeFlushTestHit(t, limiter, "k")
	takeFlushTestHit(t, limiter, "k")
	release()
	if err := <-flushDone; err != nil {
		t.Fatal(err)
	}

	state := windowStateForTest(t, limiter, redisKey)
	if state.redisKnown != 3 || state.localDelta != 2 || state.flushDelta != 0 {
		t.Fatalf("after in-flight Takes %+v, want redis_known 3, local_delta 2, not in flight", state)
	}
}

// TestFlush_ReplyWithoutOneSlotPerKeyFailsBatch: a reply whose slot count differs from the keys sent fails every key of that EVAL.
func TestFlush_ReplyWithoutOneSlotPerKeyFailsBatch(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	takeFlushTestHit(t, limiter, "a")
	takeFlushTestHit(t, limiter, "b")
	fake.replyNextFlush("*1\r\n:5\r\n")

	flushErr := limiter.flushPending(context.Background())
	if flushErr == nil || !strings.Contains(flushErr.Error(), simpleredis.RedisIssue) {
		t.Fatalf("flush error %v, want %s", flushErr, simpleredis.RedisIssue)
	}
	for _, key := range []string{"a", "b"} {
		state := windowStateForTest(t, limiter, currentWindowKeyForTest(t, limiter, key))
		if state.localDelta != 1 || state.flushDelta != 0 || state.redisKnown != 0 {
			t.Fatalf("%s after mismatched reply %+v, want local_delta 1 kept", key, state)
		}
	}
}

// TestFlush_NonIntegerSlotKeepsCause: a slot that is neither an integer nor an err: report fails that key only and wraps the parse error.
func TestFlush_NonIntegerSlotKeepsCause(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	takeFlushTestHit(t, limiter, "a")
	takeFlushTestHit(t, limiter, "b")
	fake.replyNextFlush("*2\r\n$1\r\nx\r\n:7\r\n")

	flushErr := limiter.flushPending(context.Background())
	if flushErr == nil || !strings.Contains(flushErr.Error(), simpleredis.RedisIssue) || !errors.Is(flushErr, strconv.ErrSyntax) {
		t.Fatalf("flush error %v, want %s wrapping a syntax error", flushErr, simpleredis.RedisIssue)
	}
	keys := fake.lastFlushBatchKeys()
	if len(keys) != 2 {
		t.Fatalf("flush EVAL KEYS %v, want two", keys)
	}
	unreadable := windowStateForTest(t, limiter, keys[0])
	if unreadable.localDelta != 1 || unreadable.flushDelta != 0 {
		t.Fatalf("key with the bad slot %+v, want local_delta 1 kept", unreadable)
	}
	applied := windowStateForTest(t, limiter, keys[1])
	if applied.redisKnown != 7 || applied.localDelta != 0 {
		t.Fatalf("key with slot 7 %+v, want redis_known 7, local_delta 0", applied)
	}
}

// TestFlush_WholeEvalFailureKeepsEveryDelta: a flush EVAL that fails as a whole keeps every delta, and the next flush sends each once.
func TestFlush_WholeEvalFailureKeepsEveryDelta(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	ctx := context.Background()
	takeFlushTestHit(t, limiter, "a")
	takeFlushTestHit(t, limiter, "b")
	fake.replyNextFlush("-ERR engine refused the script\r\n")

	if err := limiter.flushPending(ctx); err == nil {
		t.Fatal("flush error nil, want the EVAL failure")
	}
	if storedOutageErrForTest(limiter) == nil {
		t.Fatal("whole EVAL failure not stored")
	}
	for _, key := range []string{"a", "b"} {
		state := windowStateForTest(t, limiter, currentWindowKeyForTest(t, limiter, key))
		if state.localDelta != 1 || state.flushDelta != 0 {
			t.Fatalf("%s after failed EVAL %+v, want local_delta 1 kept", key, state)
		}
	}

	if err := limiter.flushPending(ctx); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"a", "b"} {
		if got := fakeStoredCountForTest(t, fake, currentWindowKeyForTest(t, limiter, key)); got != 1 {
			t.Fatalf("Redis %s = %d, want 1", key, got)
		}
	}
}

// TestFlush_ManyKeysFewerEvalsThanKeys: 1000 dirty keys flush in fewer EVALs than keys, none above 1024 keys, each delta once.
func TestFlush_ManyKeysFewerEvalsThanKeys(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	limiter := newFlushTestLimiter(t, addr)
	const dirtyKeys = 1000
	for i := 0; i < dirtyKeys; i++ {
		takeFlushTestHit(t, limiter, "ip:"+strconv.Itoa(i))
	}

	if err := limiter.flushPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	counts := fake.flushEvalKeyCounts()
	if len(counts) >= dirtyKeys {
		t.Fatalf("flush EVALs %d, want fewer than %d keys", len(counts), dirtyKeys)
	}
	if wantEvals := (dirtyKeys + flushBatchKeys - 1) / flushBatchKeys; len(counts) != wantEvals {
		t.Fatalf("flush EVALs %d, want %d batches of at most %d keys", len(counts), wantEvals, flushBatchKeys)
	}
	sent := 0
	for _, keysInEval := range counts {
		if keysInEval > 1024 {
			t.Fatalf("flush EVAL with %d keys, want at most 1024", keysInEval)
		}
		if keysInEval > flushBatchKeys {
			t.Fatalf("flush EVAL with %d keys, want at most flushBatchKeys %d", keysInEval, flushBatchKeys)
		}
		sent += keysInEval
	}
	if sent != dirtyKeys {
		t.Fatalf("keys sent %d, want %d", sent, dirtyKeys)
	}
	for i := 0; i < dirtyKeys; i++ {
		redisKey := currentWindowKeyForTest(t, limiter, "ip:"+strconv.Itoa(i))
		if got := fakeStoredCountForTest(t, fake, redisKey); got != 1 {
			t.Fatalf("Redis %s = %d, want 1", redisKey, got)
		}
	}
}
