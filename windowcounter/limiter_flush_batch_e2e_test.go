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
	runForEachLiveEngine(t, "WINDOWCOUNTER_LIVE_REDIS", "WINDOWCOUNTER_LIVE_DRAGONFLY", runLiveFlushBatchSeveralKeys)
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
			if _, _, takeErr := limiter.Take(ctx, key, 1000, window); takeErr != nil {
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
