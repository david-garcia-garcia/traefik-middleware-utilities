package tokenbucket

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestTraefik14328_RedisConcurrentFirstRequests proves 16 simultaneous first Allows on a new Redis or
// Dragonfly key admit at most burst (Traefik #14328: the Get/Set race does not happen here because Redis
// runs one atomic Eval).
func TestTraefik14328_RedisConcurrentFirstRequests(t *testing.T) {
	runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", runRedisConcurrentFirstRequests14328)
}

// runRedisConcurrentFirstRequests14328 runs 10 rounds of 16 concurrent first Allows against one live engine.
func runRedisConcurrentFirstRequests14328(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient14328(t, addr)
	limiter, err := NewRedis(client, 1.0/60, 5, 500*time.Millisecond, 120*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// One limiter, a new opaque key per round, so hashes do not share burst.
	maxAdmits := 0
	for round := 0; round < 10; round++ {
		key := liveKey14328(t, "source-"+strconv.Itoa(round))
		got := concurrentFirstAdmits14328(t, limiter.Allow, key, 16, 5)
		if got > maxAdmits {
			maxAdmits = got
		}
	}
	t.Logf("max admits across 10 rounds: %d", maxAdmits)
}

// waitLiveClient14328 dials addr with a 16-socket pool so 16 first requests can overlap, then waits until Eval succeeds.
func waitLiveClient14328(t *testing.T, addr string) *simpleredis.SimpleRedis {
	t.Helper()
	client, err := simpleredis.New(simpleredis.Config{
		Host:         addr,
		PoolSize:     16,
		MaxIdleConns: 16,
		PoolTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Same 15s Eval wait as waitLiveClient, on this issue's pool Config.
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Eval(context.Background(), "return 1", simpleredis.ScriptSHA1Hex("return 1"), nil, nil)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// liveKey14328 returns an opaque key unique to this test and suffix so rounds do not share a hash.
func liveKey14328(t *testing.T, suffix string) string {
	t.Helper()
	return t.Name() + ":" + suffix
}
