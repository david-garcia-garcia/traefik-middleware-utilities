package simpleredis

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// TestLive_IoTimeoutRedisAndDragonfly proves a configured IoTimeout on BLPOP against live Redis and Dragonfly.
func TestLive_IoTimeoutRedisAndDragonfly(t *testing.T) {
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
			runLiveIoTimeout(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

// runLiveIoTimeout stalls one connection with BLPOP on a unique empty list until IoTimeout fires.
func runLiveIoTimeout(t *testing.T, addr string) {
	t.Helper()
	waitLiveClient(t, addr)

	client := &SimpleRedis{}
	client.InitWithOptions(addr, "", "", Options{IoTimeout: 50 * time.Millisecond})
	key := t.Name() + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	started := time.Now()
	_, err := client.exec([]byte("BLPOP"), []byte(key), []byte("10"))
	elapsed := time.Since(started)
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("BLPOP = %v, want %s", err, RedisTimeout)
	}
	if elapsed >= 10*time.Second {
		t.Fatalf("elapsed %v, want well before 10s", elapsed)
	}
}

// waitLiveClient retries Incr until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) {
	t.Helper()
	client := &SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Incr("simpleredis-live-probe")
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
