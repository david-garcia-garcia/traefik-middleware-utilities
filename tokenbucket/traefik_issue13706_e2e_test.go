package tokenbucket

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// liveRunNonce13706 is one process-wide nonce so live keys cannot collide with TestLive_Limiter (bare t.Name()) or a parallel run.
var liveRunNonce13706 = strconv.FormatInt(time.Now().UnixNano(), 10)

// liveKey13706 is a Redis key unique to this issue's live test: run nonce + t.Name() + suffix.
func liveKey13706(t *testing.T) string {
	t.Helper()
	return "tb13706:" + liveRunNonce13706 + ":" + t.Name() + ":scope"
}

// TestTraefik13706_RedisShare: two *Redis on one key admit burst in total (state lives on the server).
func TestTraefik13706_RedisShare(t *testing.T) {
	runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", runTraefik13706LiveShare)
}

// runTraefik13706LiveShare drains two Redis limiters on one liveKey13706 against addr.
func runTraefik13706LiveShare(t *testing.T, addr string) {
	t.Helper()
	now := time.Unix(time.Now().Unix(), 0)
	key := liveKey13706(t)
	attempts := int(burst13706) + 5
	routerA := newFrozenRedis13706(t, addr, now)
	routerB := newFrozenRedis13706(t, addr, now)
	admittedA := countAdmits13706(t, attempts, func() (Status, error) {
		return routerA.Allow(context.Background(), key)
	})
	admittedB := countAdmits13706(t, attempts, func() (Status, error) {
		return routerB.Allow(context.Background(), key)
	})
	total := admittedA + admittedB
	if admittedA != int(burst13706) || admittedB != 0 {
		t.Fatalf("two *Redis on one key: first admitted %d, second admitted %d, want %d then 0", admittedA, admittedB, burst13706)
	}
	if total != int(burst13706) {
		t.Fatalf("two *Redis on one key admitted %d, want burst %d", total, burst13706)
	}
}

// newFrozenRedis13706 builds a live Redis limiter with maxDelay 0 and a frozen clock so refill cannot hide the burst cap.
func newFrozenRedis13706(t *testing.T, addr string, now time.Time) *Redis {
	t.Helper()
	limiter, err := NewRedis(waitLiveClient(t, addr), 1, burst13706, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	limiter.SetNowForTest(func() time.Time { return now })
	return limiter
}
