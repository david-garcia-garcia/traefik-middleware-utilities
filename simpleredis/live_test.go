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

// runLivePoolBackend Inits against a live engine then holds the only in-use turn
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

// waitLiveSimpleRedis Inits a one-slot client and waits until Set against addr succeeds.
func waitLiveSimpleRedis(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := &SimpleRedis{}
	client.poolSize = 1
	client.poolTimeout = 80 * time.Millisecond
	client.Init(addr, "", "")
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
