package leakybucket

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
		{"redis", os.Getenv("LEAKYBUCKET_LIVE_REDIS")},
		{"dragonfly", os.Getenv("LEAKYBUCKET_LIVE_DRAGONFLY")},
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
		t.Skip("LEAKYBUCKET_LIVE_REDIS and LEAKYBUCKET_LIVE_DRAGONFLY unset")
	}
}

// liveKey prefixes t.Name so this package's Redis keys cannot collide with tokenbucket live tests on the shared CI engines.
func liveKey(t *testing.T) string {
	t.Helper()
	return "leakybucket:" + t.Name()
}

// runLiveBackend runs exact pour-to-cap then leak, buffered two-instance, and memory/Redis agreement against one engine.
func runLiveBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	now := time.Unix(time.Now().Unix(), 0)
	t.Run("exactPourThenLeak", func(t *testing.T) {
		limiter, err := NewRedis(client, 1, 3, 0, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		limiter.SetNowForTest(func() time.Time { return now })
		key := liveKey(t)
		for i := 0; i < 3; i++ {
			allowed, _, _, takeErr := limiter.Take(key)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatalf("pour %d denied", i+1)
			}
		}
		allowed, _, _, err := limiter.Take(key)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatal("want deny at cap")
		}
		nowAfterDrain := now.Add(3 * time.Second)
		limiter.SetNowForTest(func() time.Time { return nowAfterDrain })
		allowed, _, _, err = limiter.Take(key)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatal("take after drain denied")
		}
	})
	t.Run("bufferedTwoInstances", func(t *testing.T) {
		aClient := waitLiveClient(t, addr)
		bClient := waitLiveClient(t, addr)
		a, err := NewRedis(aClient, 1, 3, time.Hour, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NewRedis(bClient, 1, 3, time.Hour, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		a.SetNowForTest(func() time.Time { return now })
		b.SetNowForTest(func() time.Time { return now })
		key := liveKey(t)
		if allowed, _, _, takeErr := a.Take(key); takeErr != nil || !allowed {
			t.Fatalf("a take: allowed %v err %v", allowed, takeErr)
		}
		if allowed, _, _, takeErr := b.Take(key); takeErr != nil || !allowed {
			t.Fatalf("b take: allowed %v err %v", allowed, takeErr)
		}
		a.Sleep()
		b.Sleep()
		reader, err := NewRedis(aClient, 1, 3, 0, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		reader.SetNowForTest(func() time.Time { return now })
		level, err := reader.Level(key)
		if err != nil {
			t.Fatal(err)
		}
		if level != 2 {
			t.Fatalf("shared water %v want 2", level)
		}
		a.Close()
		b.Close()
	})
	t.Run("memoryAgrees", func(t *testing.T) {
		mem, err := NewMemory(2, 2, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		red, err := NewRedis(client, 2, 2, 0, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		mem.SetNowForTest(func() time.Time { return now })
		red.SetNowForTest(func() time.Time { return now })
		key := liveKey(t)
		for i := 0; i < 4; i++ {
			mAllowed, mLevel, _, mErr := mem.Take(key)
			if mErr != nil {
				t.Fatal(mErr)
			}
			rAllowed, rLevel, _, rErr := red.Take(key)
			if rErr != nil {
				t.Fatal(rErr)
			}
			if mAllowed != rAllowed {
				t.Fatalf("step %d allowed mem %v redis %v", i, mAllowed, rAllowed)
			}
			if waterClass(mLevel, 2) != waterClass(rLevel, 2) {
				t.Fatalf("step %d water mem %v redis %v", i, mLevel, rLevel)
			}
		}
	})
}

// waitLiveClient retries Eval until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *simpleredis.SimpleRedis {
	t.Helper()
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Eval("return 1", nil, nil)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
