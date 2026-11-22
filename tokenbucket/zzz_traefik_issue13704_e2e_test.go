package tokenbucket

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// liveKey13704 is a per-test Redis key for issue 13704.
func liveKey13704(t *testing.T, suffix string) string {
	t.Helper()
	return "tb-13704:" + t.Name() + ":" + suffix
}

// evalKey13704 runs a test script on KEYS[1] so Dragonfly accepts the EVAL.
func evalKey13704(t *testing.T, client *simpleredis.SimpleRedis, script, key string) [][]byte {
	t.Helper()
	values, err := client.Eval(context.Background(), script, simpleredis.ScriptSHA1Hex(script), []string{key}, nil)
	if err != nil {
		t.Fatalf("eval on %s: %v", key, err)
	}
	return values
}

// TestTraefik13704_RedisBucketExpires: https://github.com/traefik/traefik/issues/13704 on a live server.
// The Redis side only reclaims idle sources if the script sets an expiry on every write. After Allow
// the bucket must carry a TTL in (0, ttl].
func TestTraefik13704_RedisBucketExpires(t *testing.T) {
	runForEachLiveEngine(t, func(t *testing.T, addr string) {
		t.Helper()
		client := waitLiveClient(t, addr)
		const ttl = 7 * time.Second
		limiter, err := NewRedis(client, 15, 10, 0, ttl)
		if err != nil {
			t.Fatal(err)
		}
		key := liveKey13704(t, "source")
		for i := 0; i < 3; i++ {
			status, allowErr := limiter.Allow(context.Background(), key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !status.Allowed {
				t.Fatalf("Allow %d denied, want admit while the bucket has tokens", i+1)
			}
		}
		values := evalKey13704(t, client, "return tostring(redis.call('pttl', KEYS[1]))", key)
		if len(values) == 0 {
			t.Fatal("PTTL eval returned no values")
		}
		pttl, err := strconv.ParseInt(string(values[0]), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PTTL after Allow: %d ms", pttl)
		if pttl <= 0 || pttl > ttl.Milliseconds() {
			t.Fatalf("bucket PTTL %d ms, want in (0, %d]: an idle source would never be reclaimed", pttl, ttl.Milliseconds())
		}
	})
}
