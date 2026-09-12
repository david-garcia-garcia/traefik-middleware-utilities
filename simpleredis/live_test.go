package simpleredis

import (
	"os"
	"testing"
	"time"
)

func TestLive_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	backends := []struct {
		name string
		addr string
	}{
		{"redis", os.Getenv("SIMPLEREDIS_LIVE_REDIS")},
		{"dragonfly", os.Getenv("SIMPLEREDIS_LIVE_DRAGONFLY")},
	}
	anyAddr := false
	for _, backend := range backends {
		if backend.addr == "" {
			continue
		}
		anyAddr = true
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			runLivePoolBackend(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

// runLivePoolBackend builds a live client then holds the only in-use turn
// so a waiter is redis:unreachable without a Lua BUSY on the shared CI Redis.
func runLivePoolBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("waiterIsUnreachable", func(t *testing.T) {
		conn, err := client.borrow()
		if err != nil {
			t.Fatalf("borrow: %v", err)
		}
		defer client.release(conn, true)
		err = client.Set("simpleredis-live-waiter", []byte("1"), 60)
		if err == nil || err.Error() != RedisUnreachable {
			t.Fatalf("waiter Set = %v, want %s", err, RedisUnreachable)
		}
	})
}

// waitLiveSimpleRedis builds a one-slot client and waits until Set against addr succeeds.
func waitLiveSimpleRedis(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := New(Config{Host: addr, PoolSize: 1, PoolTimeout: 80 * time.Millisecond})
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := client.Set("simpleredis-live-probe", []byte("1"), 60); err == nil {
			return client
		} else if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// liveEngine is one skip-if-unset dest Redis or Dragonfly address.
type liveEngine struct {
	name string
	addr string
}

// TestLiveSelectOutOfRange_RedisAndDragonfly proves SELECT 99 on dest engines is not pooled.
func TestLiveSelectOutOfRange_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	runLivePerEngine(t, []liveEngine{
		{"redis", os.Getenv("SIMPLEREDIS_LIVE_REDIS")},
		{"dragonfly", os.Getenv("SIMPLEREDIS_LIVE_DRAGONFLY")},
	}, "SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset", func(t *testing.T, addr string) {
		client := New(Config{Host: addr, Database: "99"})
		t.Cleanup(client.Close)
		_, err := client.Get("k")
		if err == nil || err.Error() != "ERR DB index is out of range" {
			t.Fatalf("Get = %v, want ERR DB index is out of range", err)
		}
		if len(client.idleConns) != 0 {
			t.Fatalf("idle = %d, want 0", len(client.idleConns))
		}
	})
}

// TestLiveWrongPassword_RedisAndDragonfly proves WRONGPASS on requirepass engines maps to redis:noauth.
func TestLiveWrongPassword_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	runLivePerEngine(t, []liveEngine{
		{"redis", os.Getenv("SIMPLEREDIS_LIVE_REDIS_AUTH")},
		{"dragonfly", os.Getenv("SIMPLEREDIS_LIVE_DRAGONFLY_AUTH")},
	}, "SIMPLEREDIS_LIVE_REDIS_AUTH and SIMPLEREDIS_LIVE_DRAGONFLY_AUTH unset", func(t *testing.T, addr string) {
		client := New(Config{Host: addr, Pass: "wrong-password"})
		t.Cleanup(client.Close)
		_, err := client.Get("k")
		if err == nil || err.Error() != RedisNoAuth {
			t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
		}
		if len(client.idleConns) != 0 {
			t.Fatalf("idle = %d, want 0", len(client.idleConns))
		}
	})
}

// runLivePerEngine runs fn against each engine with an address; skips when none are set.
func runLivePerEngine(t *testing.T, engines []liveEngine, skipAll string, run func(*testing.T, string)) {
	t.Helper()
	anyAddr := false
	for _, engine := range engines {
		if engine.addr == "" {
			continue
		}
		anyAddr = true
		engine := engine
		t.Run(engine.name, func(t *testing.T) {
			run(t, engine.addr)
		})
	}
	if !anyAddr {
		t.Skip(skipAll)
	}
}
