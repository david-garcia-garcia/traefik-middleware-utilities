package windowcounter

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// TestLive_FlushBatchSeveralKeys proves one buffered flush that carries several keys per EVAL (more keys than one batch)
// applies every key's delta exactly once, against live Redis and Dragonfly.
func TestLive_FlushBatchSeveralKeys(t *testing.T) {
	runForEachLiveEngine(t, runLiveFlushBatchSeveralKeys)
}

// runLiveFlushBatchSeveralKeys flushes two hits on flushBatchKeys+10 keys against one engine and reads every count back.
func runLiveFlushBatchSeveralKeys(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer limiter.Close()
	// Frozen clock so every Take and the key lookup below use the same window.
	now := time.Now()
	limiter.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	window := time.Minute
	keyPrefix := t.Name() + ":" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
	// Two hits on each of flushBatchKeys+10 keys, so the flush needs two EVALs and the first carries a full batch.
	keyCount := flushBatchKeys + 10
	redisKeys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		key := keyPrefix + strconv.Itoa(i)
		for hit := 0; hit < 2; hit++ {
			if _, takeErr := limiter.Take(ctx, key, 1000, window); takeErr != nil {
				t.Fatal(takeErr)
			}
		}
		sliding, slidingErr := limiter.slidingAt(key, window)
		if slidingErr != nil {
			t.Fatal(slidingErr)
		}
		redisKeys[i] = sliding.currentKey
	}

	if flushErr := limiter.flushPending(ctx); flushErr != nil {
		t.Fatal(flushErr)
	}
	for _, redisKey := range redisKeys {
		count, getErr := limiter.getCount(ctx, redisKey)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if count != 2 {
			t.Fatalf("Redis %s = %d after one flush, want 2", redisKey, count)
		}
	}
}

// TestLive_FlushBatchFailedKeyKeepsDelta proves the flush script's per-key redis.pcall on a live engine: a key whose INCRBY
// fails keeps its delta and stores the error, the other keys of that EVAL apply once, and the next flush sends only the
// failed key, on Redis and Dragonfly.
func TestLive_FlushBatchFailedKeyKeepsDelta(t *testing.T) {
	runForEachLiveEngine(t, runLiveFlushBatchFailedKeyKeepsDelta)
}

// runLiveFlushBatchFailedKeyKeepsDelta puts a non-integer value on one window key of a three-key flush against one engine.
func runLiveFlushBatchFailedKeyKeepsDelta(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer limiter.Close()
	now := time.Now()
	limiter.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	window := time.Minute
	keyPrefix := t.Name() + ":" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
	redisKeyOf := map[string]string{}
	for _, name := range []string{"applied-a", "failing", "applied-b"} {
		key := keyPrefix + name
		if _, takeErr := limiter.Take(ctx, key, 1000, window); takeErr != nil {
			t.Fatal(takeErr)
		}
		sliding, slidingErr := limiter.slidingAt(key, window)
		if slidingErr != nil {
			t.Fatal(slidingErr)
		}
		redisKeyOf[name] = sliding.currentKey
	}
	failingKey := redisKeyOf["failing"]
	// INCRBY on a non-integer string fails inside the script for this key only.
	if setErr := client.Set(ctx, failingKey, []byte("not-a-number"), 60); setErr != nil {
		t.Fatal(setErr)
	}

	if flushErr := limiter.flushPending(ctx); flushErr == nil {
		t.Fatal("flush with a non-integer key returned nil, want that key's INCRBY error")
	}
	if storedOutageErrForTest(limiter) == nil {
		t.Fatal("failed key's error not stored")
	}
	if state := windowStateForTest(t, limiter, failingKey); state.localDelta != 1 || state.flushDelta != 0 {
		t.Fatalf("failing key localDelta %d flushDelta %d, want 1 and 0", state.localDelta, state.flushDelta)
	}
	for _, name := range []string{"applied-a", "applied-b"} {
		count, getErr := limiter.getCount(ctx, redisKeyOf[name])
		if getErr != nil {
			t.Fatal(getErr)
		}
		if count != 1 {
			t.Fatalf("Redis %s = %d after the batch, want 1", name, count)
		}
	}

	// Once the key holds an integer again, the next flush sends only the failed key's delta.
	if delErr := client.Del(ctx, failingKey); delErr != nil {
		t.Fatal(delErr)
	}
	if flushErr := limiter.flushPending(ctx); flushErr != nil {
		t.Fatal(flushErr)
	}
	for name, redisKey := range redisKeyOf {
		count, getErr := limiter.getCount(ctx, redisKey)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if count != 1 {
			t.Fatalf("Redis %s = %d after the retry flush, want 1 (each delta once)", name, count)
		}
	}
}
