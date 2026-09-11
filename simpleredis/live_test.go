package simpleredis

import (
	"os"
	"testing"
)

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
		var client SimpleRedis
		client.Init(addr, "", "99")
		_, err := client.Get("k")
		if err == nil || err.Error() != "ERR DB index is out of range" {
			t.Fatalf("Get = %v, want ERR DB index is out of range", err)
		}
		if len(client.idle) != 0 {
			t.Fatalf("idle = %d, want 0", len(client.idle))
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
		var client SimpleRedis
		client.Init(addr, "wrong-password", "")
		_, err := client.Get("k")
		if err == nil || err.Error() != RedisNoAuth {
			t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
		}
		if len(client.idle) != 0 {
			t.Fatalf("idle = %d, want 0", len(client.idle))
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
