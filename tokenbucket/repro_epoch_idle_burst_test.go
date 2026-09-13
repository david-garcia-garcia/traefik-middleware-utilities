package tokenbucket

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRepro_NewKeyFillsToBurstAtEpoch proves a new key is filled to burst before
// consuming 1, including Unix epoch (elapsed from last=0 is 0) and when
// elapsed*rate from epoch is still below a huge burst. Existing tests freeze at
// Unix(1_700_000_000) which hides both.
func TestRepro_NewKeyFillsToBurstAtEpoch(t *testing.T) {
	t.Run("epoch_clock", func(t *testing.T) {
		const burst int64 = 5
		limiter, err := NewMemory(1, burst, time.Hour, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Unix(0, 0)
		if now.UnixMicro() != 0 {
			now = time.Unix(0, 1)
		}
		limiter.SetNowForTest(func() time.Time { return now })
		t.Logf("now UnixMicro=%d (zero-value UnixMicro=%d)", now.UnixMicro(), time.Time{}.UnixMicro())

		for i := 0; i < int(burst); i++ {
			allowed, wait, allowErr := limiter.Allow(context.Background(), "new-key")
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			tokens := limiter.buckets["new-key"].tokens
			if !allowed {
				t.Fatalf("Allow %d of burst %d denied (want allowed true when filled to burst)", i+1, burst)
			}
			if tokens < 0 {
				t.Fatalf("Allow %d of burst %d delayed: tokens=%v wait=%v last=%d (want filled to burst so tokens>=0 after consume)",
					i+1, burst, tokens, wait, limiter.buckets["new-key"].last)
			}
		}

		sixth, wait, sixthErr := limiter.Allow(context.Background(), "new-key")
		if sixthErr != nil {
			t.Fatal(sixthErr)
		}
		if !sixth {
			t.Fatal("6th Allow: want allowed true when maxDelay is 1h")
		}
		if wait <= 0 {
			t.Fatalf("6th Allow wait %v want > 0", wait)
		}
	})

	t.Run("huge_burst_elapsed_below_burst", func(t *testing.T) {
		const burst int64 = 1_000_000_000_000
		limiter, err := NewMemory(1, burst, time.Hour, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Unix(1_700_000_000, 0)
		limiter.SetNowForTest(func() time.Time { return now })

		allowed, _, allowErr := limiter.Allow(context.Background(), "new-key")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed {
			t.Fatal("first Allow denied")
		}
		tokens := limiter.buckets["new-key"].tokens
		wantAfterConsume := float64(burst) - 1
		elapsedSeconds := float64(now.UnixMicro()) / 1e6
		t.Logf("tokens after first Allow=%v want=%v elapsedSecondsFromEpoch=%v", tokens, wantAfterConsume, elapsedSeconds)
		if tokens < wantAfterConsume {
			t.Fatalf("new key not filled to burst: tokens after first Allow=%v want %v (elapsed*rate from last=0 is %v, far below burst %d)",
				tokens, wantAfterConsume, tokens+1, burst)
		}
	})

	t.Run("redis_fake_epoch_clock", func(t *testing.T) {
		assertRedisFakeFilledToBurst(t, time.Unix(0, 0), 5)
	})

	t.Run("redis_fake_huge_burst_elapsed_below_burst", func(t *testing.T) {
		assertRedisFakeFilledToBurst(t, time.Unix(1_700_000_000, 0), 1_000_000_000_000)
	})

	t.Run("lua_empty_hash_seed_in_script", func(t *testing.T) {
		if !strings.Contains(allowScript, "bucket.tokens = burst") {
			t.Fatal("allowScript missing empty-hash seed bucket.tokens = burst")
		}
		if !strings.Contains(allowScript, "bucket.last = t") {
			t.Fatal("allowScript missing empty-hash seed bucket.last = t")
		}
	})
}

// assertRedisFakeFilledToBurst calls Redis Allow on a missing hash and checks remaining tokens.
func assertRedisFakeFilledToBurst(t *testing.T, now time.Time, burst int64) {
	t.Helper()
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := NewRedis(client, 1, burst, time.Hour, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, allowErr := limiter.Allow(context.Background(), "new-key")
	if allowErr != nil {
		t.Fatal(allowErr)
	}
	if !allowed {
		t.Fatal("first Allow denied")
	}
	fake.mu.Lock()
	hash := fake.hashes["new-key"]
	fake.mu.Unlock()
	if hash == nil {
		t.Fatal("missing hash after Allow")
	}
	tokens, parseErr := strconv.ParseFloat(hash.tokens, 64)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	wantAfterConsume := float64(burst) - 1
	t.Logf("fake redis tokens after first Allow=%v want=%v last=%s", tokens, wantAfterConsume, hash.last)
	if tokens < wantAfterConsume {
		t.Fatalf("fake missing hash not filled to burst: tokens=%v want %v last=%s", tokens, wantAfterConsume, hash.last)
	}
}
