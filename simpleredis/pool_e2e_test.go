package simpleredis

import (
	"context"
	"testing"
	"time"
)

// TestLive_PoolWait proves a waiter is redis:unreachable when the only in-use turn is held.
func TestLive_PoolWait(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", runLivePoolBackend)
}

// TestLive_PeerCloseEOFRedial proves live Redis and Dragonfly recover after CLIENT KILL of the pooled socket.
func TestLive_PeerCloseEOFRedial(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", runLivePeerCloseRecovery)
}

// TestLive_SelectOutOfRange proves SELECT 99 on dest engines is not pooled.
func TestLive_SelectOutOfRange(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", func(t *testing.T, addr string) {
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

// TestLive_WrongPassword proves WRONGPASS on requirepass engines maps to redis:noauth.
func TestLive_WrongPassword(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS_AUTH", "SIMPLEREDIS_LIVE_DRAGONFLY_AUTH", func(t *testing.T, addr string) {
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

// runLivePoolBackend builds a live client then holds the only in-use turn
// so a waiter is redis:unreachable without a Lua BUSY on the shared CI Redis.
func runLivePoolBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("waiterIsUnreachable", func(t *testing.T) {
		conn, err := client.borrow(context.Background(), time.Now().Add(time.Second))
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

// runLivePeerCloseRecovery Gets, CLIENT KILLs that pooled ADDR (or ID), then Gets on a new dial.
func runLivePeerCloseRecovery(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	key := "simpleredis-live-eof:" + t.Name()
	if err := client.Set(key, []byte("ok"), 60); err != nil {
		t.Fatal(err)
	}
	got, err := client.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("Get = %q, want ok", got)
	}

	client.idleConnsMu.Lock()
	if len(client.idleConns) != 1 {
		client.idleConnsMu.Unlock()
		t.Fatalf("idle after Get = %d, want 1", len(client.idleConns))
	}
	killed := client.idleConns[0]
	client.idleConnsMu.Unlock()

	killPooledIdleAddrOrIDForTest(t, client)

	got, err = client.Get(key)
	if err != nil {
		t.Fatalf("Get after CLIENT KILL: %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("Get after CLIENT KILL = %q, want ok", got)
	}

	client.idleConnsMu.Lock()
	defer client.idleConnsMu.Unlock()
	for _, conn := range client.idleConns {
		if conn == killed {
			t.Fatal("killed socket was reused")
		}
	}
}
