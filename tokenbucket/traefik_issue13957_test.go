package tokenbucket

import (
	"context"
	"math"
	"testing"
	"time"
)

// traefikBucketTTL13957 is Traefik's idle TTL for a rate in tokens per second (traefik/traefik#13957).
func traefikBucketTTL13957(ratePerSecond float64) time.Duration {
	ttl := 1
	if ratePerSecond >= 1 {
		ttl++
	} else if ratePerSecond > 0 {
		ttl += int(1 / ratePerSecond)
	}
	return time.Duration(ttl) * time.Second
}

// refillTTL13957 is the whole seconds an empty bucket needs to fill when maxDelay is 0.
func refillTTL13957(ratePerSecond float64, burst int64) time.Duration {
	return time.Duration(math.Ceil(float64(burst)/ratePerSecond)) * time.Second
}

// countAdmits13957 counts Allowed results until the first deny or attempts runs out.
func countAdmits13957(t *testing.T, attempts int, allow func() (bool, error)) int {
	t.Helper()
	admitted := 0
	for i := 0; i < attempts; i++ {
		ok, err := allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i+1, err)
		}
		if !ok {
			return admitted
		}
		admitted++
	}
	return admitted
}

// TestTraefik13957_MemoryIdleBucketRecreatedFull: traefik/traefik#13957.
// 30 req/min, burst 30. After a 5s pause a correct bucket admits 2, not a fresh 30.
func TestTraefik13957_MemoryIdleBucketRecreatedFull(t *testing.T) {
	const (
		rate  = 0.5
		burst = int64(30)
		pause = 5 * time.Second
	)
	wantAfterPause := int(rate * pause.Seconds())

	cases := []struct {
		name string
		ttl  time.Duration
	}{
		{name: "control_ttl_covers_refill", ttl: refillTTL13957(rate, burst)},
		{name: "traefik_ttl", ttl: traefikBucketTTL13957(rate)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter, err := NewMemory(rate, burst, 0, tc.ttl)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1_700_000_000, 0)
			limiter.SetNowForTest(func() time.Time { return now })
			allow := func() (bool, error) {
				status, allowErr := limiter.Allow(context.Background(), "tenant-a")
				return status.Allowed, allowErr
			}

			if got := countAdmits13957(t, int(burst)+10, allow); got != int(burst) {
				t.Fatalf("first burst admitted %d, want %d", got, burst)
			}
			now = now.Add(pause)
			got := countAdmits13957(t, int(burst)+10, allow)
			if got != wantAfterPause {
				t.Fatalf("after a %v pause admitted %d, want %d", pause, got, wantAfterPause)
			}
		})
	}
}

// TestTTLFloor13957 proves the stored idle lifetime: short ttl raised, long ttl kept, maxDelay included, overflow clamped.
func TestTTLFloor13957(t *testing.T) {
	t.Run("short raised", func(t *testing.T) {
		limiter, err := NewMemory(0.5, 30, 0, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := limiter.clock.ttl, 60*time.Second; got != want {
			t.Fatalf("ttl %v, want %v", got, want)
		}
	})

	t.Run("long kept", func(t *testing.T) {
		limiter, err := NewMemory(0.5, 30, 0, 120*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := limiter.clock.ttl, 120*time.Second; got != want {
			t.Fatalf("ttl %v, want %v", got, want)
		}
		fake, addr := startTestFakeRedis(t)
		client := newSimpleRedisForTest(t, addr)
		redisLimiter, err := NewRedis(client, 0.5, 30, 0, 120*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if _, allowErr := redisLimiter.Allow(context.Background(), "kept"); allowErr != nil {
			t.Fatal(allowErr)
		}
		eval := fake.lastEvalCommand()
		if len(eval) < 7 || eval[6] != "120" {
			t.Fatalf("ARGV ttl %v, want 120", eval)
		}
	})

	t.Run("maxDelay included", func(t *testing.T) {
		limiter, err := NewMemory(1, 2, time.Hour, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := limiter.clock.ttl, 3602*time.Second; got != want {
			t.Fatalf("ttl %v, want %v", got, want)
		}
	})

	t.Run("overflow clamp", func(t *testing.T) {
		limiter, err := NewMemory(1e-20, math.MaxInt64, 0, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := limiter.clock.ttl, maxWholeSecondTTL; got != want {
			t.Fatalf("ttl %v, want %v", got, want)
		}
	})
}
