package simpleredis

import (
	"os"
	"sync"
	"testing"
	"time"
)

const timeWaitHoldScript = `local start = redis.call("TIME")
local startSec = tonumber(start[1])
local startUsec = tonumber(start[2])
local need = tonumber(ARGV[1])
while true do
  local now = redis.call("TIME")
  local elapsed = (tonumber(now[1]) - startSec) * 1000000 + (tonumber(now[2]) - startUsec)
  if elapsed >= need then
    break
  end
end
return 1`

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

// runLivePoolBackend proves a waiter is redis:unreachable on a live engine.
// One in-use turn (not eight Lua holds) so the shared CI Redis is not BUSY for seconds;
// Traefik Pester remains the eight-socket proof.
func runLivePoolBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("waiterIsUnreachable", func(t *testing.T) {
		started := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			close(started)
			_, _ = client.Eval(timeWaitHoldScript, nil, []string{"200000"})
		}()
		<-started
		time.Sleep(30 * time.Millisecond)
		err := client.Set("simpleredis-live-waiter", []byte("1"), 60)
		wg.Wait()
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
