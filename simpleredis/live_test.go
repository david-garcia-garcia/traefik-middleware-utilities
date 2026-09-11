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
			runLiveIdleHead(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

// runLiveIdleHead proves a stale idle head is closed while the tail stays hot on one engine.
func runLiveIdleHead(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	key := t.Name()
	if err := client.Set(key, []byte("t"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	proveIdleHeadSweep(t, client, key)
}

// waitLiveClient retries Set until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := &SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		err := client.Set("simpleredis-live-probe", []byte("1"), 60)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
