package simpleredis

import (
	"testing"
	"time"
)

// TestLive_MSetEX proves Lua MSetEX TTL, future EXAT TTL, and past EXAT miss on live Redis and Dragonfly.
func TestLive_MSetEX(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", runLiveMSetEXBackend)
}

// runLiveMSetEXBackend proves Lua MSetEX TTL landed, future EXAT TTL, and past EXAT miss on one engine.
func runLiveMSetEXBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

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
		assertLiveTTLPositive(t, client, key)
	})
	t.Run("msetexAtTTLLanded", func(t *testing.T) {
		key := t.Name()
		if err := client.MSetEXAt([]string{key}, [][]byte{[]byte("ok")}, time.Now().Unix()+90); err != nil {
			t.Fatalf("MSetEXAt: %v", err)
		}
		got, err := client.Get(key)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(got) != "ok" {
			t.Fatalf("Get = %q, want ok", got)
		}
		assertLiveTTLPositive(t, client, key)
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
