package tokenbucket

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
		{"redis", os.Getenv("TOKENBUCKET_LIVE_REDIS")},
		{"dragonfly", os.Getenv("TOKENBUCKET_LIVE_DRAGONFLY")},
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
		t.Skip("TOKENBUCKET_LIVE_REDIS and TOKENBUCKET_LIVE_DRAGONFLY unset")
	}
}

// runLiveBackend runs burst, two-instance share, and memory/Redis agreement against one engine.
func runLiveBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	now := time.Unix(time.Now().Unix(), 0)
	t.Run("burstAfterIdle", func(t *testing.T) {
		limiter, err := NewRedis(client, 1, 3, time.Hour, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 3; i++ {
			allowed, _, allowErr := limiter.Allow(key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !allowed {
				t.Fatalf("burst %d denied", i+1)
			}
		}
		allowed, wait, err := limiter.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed || wait <= 0 {
			t.Fatalf("want delayed admit, allowed %v wait %v", allowed, wait)
		}
	})
	t.Run("twoInstancesShare", func(t *testing.T) {
		aClient := waitLiveClient(t, addr)
		bClient := waitLiveClient(t, addr)
		a, err := NewRedis(aClient, 1, 3, time.Microsecond, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NewRedis(bClient, 1, 3, time.Microsecond, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		a.SetNowForTest(func() time.Time { return now })
		b.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 3; i++ {
			allowed, _, allowErr := a.Allow(key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !allowed {
				t.Fatalf("a burst %d denied", i+1)
			}
		}
		allowed, _, err := b.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatal("second instance granted another burst")
		}
	})
	t.Run("memoryAgrees", func(t *testing.T) {
		mem, err := NewMemory(2, 2, agreeMaxDelay, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		red, err := NewRedis(client, 2, 2, agreeMaxDelay, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		mem.SetNowForTest(func() time.Time { return now })
		red.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 4; i++ {
			mAllowed, mWait, mErr := mem.Allow(key)
			if mErr != nil {
				t.Fatal(mErr)
			}
			rAllowed, rWait, rErr := red.Allow(key)
			if rErr != nil {
				t.Fatal(rErr)
			}
			if mAllowed != rAllowed {
				t.Fatalf("step %d allowed mem %v redis %v", i, mAllowed, rAllowed)
			}
			if waitClass(mWait) != waitClass(rWait) {
				t.Fatalf("step %d wait mem %v redis %v", i, mWait, rWait)
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
