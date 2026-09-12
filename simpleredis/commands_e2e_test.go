package simpleredis

import (
	"testing"
	"time"
)

// TestLive_Commands proves Get/Set/Del/MGet/Incr/Expire against live Redis and Dragonfly.
func TestLive_Commands(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", runLiveCommandsBackend)
}

// runLiveCommandsBackend proves engine-success Get/Set/Del/MGet/Incr/Expire against one engine.
func runLiveCommandsBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("getHitAndMiss", func(t *testing.T) {
		key := t.Name()
		if err := client.Set(key, []byte("t"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := client.Get(key)
		if err != nil {
			t.Fatalf("Get hit: %v", err)
		}
		if string(got) != "t" {
			t.Fatalf("Get hit = %q, want t", got)
		}
		if _, err = client.Get(key + "-missing"); err == nil || err.Error() != RedisMiss {
			t.Fatalf("Get missing = %v, want %s", err, RedisMiss)
		}
	})
	t.Run("setExThenGet", func(t *testing.T) {
		key := t.Name()
		if err := client.Set(key, []byte("v"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := client.Get(key)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(got) != "v" {
			t.Fatalf("Get = %q, want v", got)
		}
		assertLiveTTLPositive(t, client, key)
	})
	t.Run("del", func(t *testing.T) {
		key := t.Name()
		if err := client.Set(key, []byte("gone"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := client.Del(key); err != nil {
			t.Fatalf("Del: %v", err)
		}
		if _, err := client.Get(key); err == nil || err.Error() != RedisMiss {
			t.Fatalf("Get after Del = %v, want %s", err, RedisMiss)
		}
	})
	t.Run("mgetHitsMissesEmpty", func(t *testing.T) {
		hitA := t.Name() + "-a"
		hitC := t.Name() + "-c"
		missB := t.Name() + "-b"
		if err := client.Set(hitA, []byte("t"), 60); err != nil {
			t.Fatalf("Set a: %v", err)
		}
		if err := client.Set(hitC, []byte("f"), 60); err != nil {
			t.Fatalf("Set c: %v", err)
		}
		got, err := client.MGet([]string{hitA, missB, hitC})
		if err != nil {
			t.Fatalf("MGet: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("MGet returned %d values, want 3", len(got))
		}
		if string(got[0]) != "t" || got[1] != nil || string(got[2]) != "f" {
			t.Fatalf("MGet = %q, want [t <nil> f]", got)
		}
		empty, err := client.MGet(nil)
		if empty != nil || err != nil {
			t.Fatalf("MGet(nil) = %v, %v, want nil, nil", empty, err)
		}
	})
	t.Run("incrThenIncrBy", func(t *testing.T) {
		key := t.Name()
		first, err := client.Incr(key)
		if err != nil {
			t.Fatalf("Incr: %v", err)
		}
		if first != 1 {
			t.Fatalf("Incr = %d, want 1", first)
		}
		got, err := client.IncrBy(key, 5)
		if err != nil {
			t.Fatalf("IncrBy: %v", err)
		}
		if got != 6 {
			t.Fatalf("IncrBy = %d, want 6", got)
		}
	})
	t.Run("expireThenTTL", func(t *testing.T) {
		key := t.Name()
		if err := client.Set(key, []byte("1"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := client.Expire(key, 90); err != nil {
			t.Fatalf("Expire: %v", err)
		}
		assertLiveTTLPositive(t, client, key)
	})
	t.Run("expireAtThenTTL", func(t *testing.T) {
		key := t.Name()
		if err := client.Set(key, []byte("1"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := client.ExpireAt(key, time.Now().Unix()+90); err != nil {
			t.Fatalf("ExpireAt: %v", err)
		}
		assertLiveTTLPositive(t, client, key)
	})
}
