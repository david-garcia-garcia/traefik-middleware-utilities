package simpleredis

import (
	"os"
	"testing"
	"time"
)

// ttlScript returns TTL for KEYS[1] so live tests can assert MSetEX expiry landed.
const ttlScript = `return redis.call('TTL', KEYS[1])`

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
			runLiveBackend(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

// runLiveBackend proves Lua MSetEX TTL landed and past EXAT is a miss on one engine.
func runLiveBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	t.Run("msetexTTLLanded", func(t *testing.T) {
		key := t.Name()
		if err := client.MSetEX([]string{key}, [][]byte{[]byte("ok")}, 60); err != nil {
			t.Fatalf("MSetEX: %v", err)
		}
		got, err := client.Get(key)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(got) != "ok" {
			t.Fatalf("Get = %q, want ok", got)
		}
		values, err := client.Eval(ttlScript, []string{key}, nil)
		if err != nil {
			t.Fatalf("TTL Eval: %v", err)
		}
		ttl, err := parseIntegerReply(values, nil)
		if err != nil {
			t.Fatalf("TTL parse: %v", err)
		}
		if ttl <= 0 {
			t.Fatalf("TTL = %d, want > 0", ttl)
		}
	})
	t.Run("pastExatMiss", func(t *testing.T) {
		key := t.Name()
		if err := client.MSetEXAt([]string{key}, [][]byte{[]byte("v")}, time.Now().Unix()-10); err != nil {
			t.Fatalf("MSetEXAt: %v", err)
		}
		if _, err := client.Get(key); err == nil || err.Error() != RedisMiss {
			t.Fatalf("Get after past EXAT = %v, want %s", err, RedisMiss)
		}
	})
}

// waitLiveClient retries Set then Get until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := &SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		err := client.Set("simpleredis-live-probe", []byte("ok"), 30)
		if err == nil {
			_, err = client.Get("simpleredis-live-probe")
		}
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
