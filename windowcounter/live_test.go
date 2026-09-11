package windowcounter

import (
	"os"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

func TestLive_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	backends := []struct {
		name string
		addr string
	}{
		{"redis", os.Getenv("WINDOWCOUNTER_LIVE_REDIS")},
		{"dragonfly", os.Getenv("WINDOWCOUNTER_LIVE_DRAGONFLY")},
	}
	anyAddr := false
	for _, backend := range backends {
		if backend.addr == "" {
			continue
		}
		anyAddr = true
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			runLiveBackend(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("WINDOWCOUNTER_LIVE_REDIS and WINDOWCOUNTER_LIVE_DRAGONFLY unset")
	}
}

// runLiveBackend runs exact N-then-deny, buffered two-client share, and sliding-at-boundary against one engine.
func runLiveBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	t.Run("exactNThenDeny", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 3
		window := time.Minute
		for i := int64(0); i < limit; i++ {
			allowed, _, takeErr := limiter.Take(key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatalf("take %d denied", i+1)
			}
		}
		allowed, _, err := limiter.Take(key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatal("want deny after N")
		}
	})
	t.Run("bufferedTwoClients", func(t *testing.T) {
		aClient := waitLiveClient(t, addr)
		bClient := waitLiveClient(t, addr)
		a, err := New(aClient, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		b, err := New(bClient, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 3
		window := time.Minute
		for i := 0; i < 2; i++ {
			allowed, _, takeErr := a.Take(key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatal("a denied early")
			}
			allowed, _, takeErr = b.Take(key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatal("b denied early")
			}
		}
		a.Sleep()
		b.Sleep()
		allowed, _, err := a.Take(key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatal("shared limit last-write-wins")
		}
		a.Close()
		b.Close()
	})
	t.Run("slidingBoundary", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		window := 10 * time.Second
		const limit int64 = 2
		start := time.Now().Unix() / 10 * 10
		now := time.Unix(start, 0).Add(9 * time.Second)
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := int64(0); i < limit; i++ {
			allowed, _, takeErr := limiter.Take(key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatalf("fill take %d denied", i+1)
			}
		}
		now = time.Unix(start, 0).Add(window)
		limiter.SetNowForTest(func() time.Time { return now })
		allowed, _, err := limiter.Take(key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatal("sliding boundary doubled like a fixed window")
		}
	})
}

// waitLiveClient retries Incr until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *simpleredis.SimpleRedis {
	t.Helper()
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Incr("windowcounter-live-probe")
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
