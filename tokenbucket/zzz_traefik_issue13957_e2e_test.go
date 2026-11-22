package tokenbucket

import (
	"context"
	"math"
	"testing"
	"time"
)

// traefikBucketTTL13957Live is Traefik's idle TTL for a rate in tokens per second (traefik/traefik#13957).
func traefikBucketTTL13957Live(ratePerSecond float64) time.Duration {
	ttl := 1
	if ratePerSecond >= 1 {
		ttl++
	} else if ratePerSecond > 0 {
		ttl += int(1 / ratePerSecond)
	}
	return time.Duration(ttl) * time.Second
}

// refillTTL13957Live is the whole seconds an empty bucket needs to fill when maxDelay is 0.
func refillTTL13957Live(ratePerSecond float64, burst int64) time.Duration {
	return time.Duration(math.Ceil(float64(burst)/ratePerSecond)) * time.Second
}

// countAdmits13957Live counts Allowed results until the first deny or attempts runs out.
func countAdmits13957Live(t *testing.T, attempts int, allow func() (bool, error)) int {
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

// liveKey13957 is one Redis key for this issue, distinct from other live tests.
func liveKey13957(t *testing.T, suffix string) string {
	t.Helper()
	return "tb-13957:" + t.Name() + ":" + suffix
}

// TestTraefik13957_RedisIdleBucketRecreatedFull: traefik/traefik#13957 on Redis, Dragonfly, and Valkey.
// A 4s pause may admit the refilled tokens plus one for scheduling slack, not a fresh burst of 30.
func TestTraefik13957_RedisIdleBucketRecreatedFull(t *testing.T) {
	runForEachLiveEngine(t, func(t *testing.T, addr string) {
		t.Helper()
		client := waitLiveClient(t, addr)
		const (
			rate  = 0.5
			burst = int64(30)
			pause = 4 * time.Second
		)
		maxAfterPause := int(rate*pause.Seconds()) + 1

		cases := []struct {
			name string
			ttl  time.Duration
		}{
			{name: "control_ttl_covers_refill", ttl: refillTTL13957Live(rate, burst)},
			{name: "traefik_ttl", ttl: traefikBucketTTL13957Live(rate)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				limiter, err := NewRedis(client, rate, burst, 0, tc.ttl)
				if err != nil {
					t.Fatal(err)
				}
				key := liveKey13957(t, "tenant-a")
				allow := func() (bool, error) {
					status, allowErr := limiter.Allow(context.Background(), key)
					return status.Allowed, allowErr
				}
				if got := countAdmits13957Live(t, int(burst)+10, allow); got != int(burst) {
					t.Fatalf("first burst admitted %d, want %d", got, burst)
				}
				time.Sleep(pause)
				got := countAdmits13957Live(t, int(burst)+10, allow)
				if got > maxAfterPause {
					t.Fatalf("after a %v pause admitted %d, want at most %d", pause, got, maxAfterPause)
				}
			})
		}
	})
}
