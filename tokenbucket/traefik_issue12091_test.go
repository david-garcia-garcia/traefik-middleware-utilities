package tokenbucket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestTraefik12091_RedisUnreachable pins traefik/traefik#12091 on this library.
// With Redis down, Traefik answers 500 and offers no fail-open or fail-closed option. Allow must surface
// the outage as simpleredis.ErrUnreachable with allowed=false so the middleware can choose. Port 1 on
// loopback refuses connections.
func TestTraefik12091_RedisUnreachable(t *testing.T) {
	client := newSimpleRedisForTest12091(t)
	defer client.Close()
	limiter := newRedisLimiterForTest12091(t, client)
	status, err := limiter.Allow(context.Background(), "client-1")
	if !errors.Is(err, simpleredis.ErrUnreachable) {
		t.Fatalf("err %v, want simpleredis.ErrUnreachable", err)
	}
	if status.Allowed {
		t.Fatal("unreachable Redis admitted the request")
	}
}

// newSimpleRedisForTest12091 returns a SimpleRedis pointed at a refusing loopback port with extra retries off.
func newSimpleRedisForTest12091(t *testing.T) *simpleredis.SimpleRedis {
	t.Helper()
	client, err := simpleredis.New(simpleredis.Config{Host: "127.0.0.1:1", MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// newRedisLimiterForTest12091 returns a Redis limiter on this refusing client with the 12091 clock (rate 1, burst 3, maxDelay 0, ttl 2s).
func newRedisLimiterForTest12091(t *testing.T, client *simpleredis.SimpleRedis) *Redis {
	t.Helper()
	limiter, err := NewRedis(client, 1, 3, 0, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}
