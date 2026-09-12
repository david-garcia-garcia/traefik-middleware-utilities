package simpleredis

import (
	"bufio"
	"net"
	"os"
	"strconv"
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
			runLiveIdleHead(t, backend.addr)
			runLiveMSetEXBackend(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

// TestLive_PeerCloseEOFRedial proves live Redis and Dragonfly recover after CLIENT KILL of the pooled socket.
func TestLive_PeerCloseEOFRedial(t *testing.T) {
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
			runLivePeerCloseRecovery(t, backend.addr)
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

// ttlScript returns TTL for KEYS[1] so live tests can assert MSetEX expiry landed.
const ttlScript = `return redis.call('TTL', KEYS[1])`

// runLiveMSetEXBackend proves Lua MSetEX TTL landed and past EXAT is a miss on one engine.
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

// runLiveIdleHead proves a stale idle head is closed while the tail stays hot on one engine.
func runLiveIdleHead(t *testing.T, addr string) {
	t.Helper()
	t.Run("staleIdleHead", func(t *testing.T) {
		client := waitLiveIdleHeadClient(t, addr)
		t.Cleanup(client.Close)
		key := t.Name()
		if err := client.Set(key, []byte("t"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		proveIdleHeadSweep(t, client, key)
	})
}

// waitLiveIdleHeadClient builds a default-pool client and waits until Set against addr succeeds.
func waitLiveIdleHeadClient(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := New(Config{Host: addr})
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := client.Set("simpleredis-live-idle-head-probe", []byte("1"), 60); err == nil {
			return client
		} else if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// killPooledIdleAddrOrIDForTest sends CLIENT KILL ADDR of the idle pooled socket from a sidecar.
// If the engine reports 0 (Docker port-publish NAT), it KILLs by that socket's CLIENT ID instead.
func killPooledIdleAddrOrIDForTest(t *testing.T, sr *SimpleRedis) {
	t.Helper()
	sr.idleConnsMu.Lock()
	if len(sr.idleConns) == 0 {
		sr.idleConnsMu.Unlock()
		t.Fatal("no idle pooled conn to kill")
	}
	pooled := sr.idleConns[0]
	addr := pooled.netConn.LocalAddr().String()
	sr.idleConnsMu.Unlock()

	killed := clientKillFromSidecarForTest(t, sr.host, "ADDR", addr)
	if killed == 0 {
		id := clientIDOnConnForTest(t, sr, pooled)
		killed = clientKillFromSidecarForTest(t, sr.host, "ID", id)
	}
	if killed == 0 {
		t.Fatalf("CLIENT KILL killed 0 clients (ADDR %s)", addr)
	}
}

// clientKillFromSidecarForTest dials host and sends CLIENT KILL <filter> <value>. Returns the killed count.
func clientKillFromSidecarForTest(t *testing.T, host, filter, value string) int {
	t.Helper()
	netConn, err := net.DialTimeout("tcp", host, defaultDialTimeout)
	if err != nil {
		t.Fatalf("sidecar dial %s: %v", host, err)
	}
	defer netConn.Close()
	if err := netConn.SetDeadline(time.Now().Add(defaultIOTimeout)); err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(netConn)
	reader := bufio.NewReader(netConn)
	if err := writeCommand(writer, [][]byte{[]byte("CLIENT"), []byte("KILL"), []byte(filter), []byte(value)}); err != nil {
		t.Fatalf("CLIENT KILL %s %s: %v", filter, value, err)
	}
	values, _, err := readReply(reader)
	if err != nil {
		t.Fatalf("CLIENT KILL %s %s: %v", filter, value, err)
	}
	if len(values) != 1 {
		t.Fatalf("CLIENT KILL reply slots %d, want 1", len(values))
	}
	n, err := strconv.Atoi(string(values[0]))
	if err != nil {
		t.Fatalf("CLIENT KILL count %q: %v", values[0], err)
	}
	return n
}

// clientIDOnConnForTest sends CLIENT ID on pooled and clears the I/O deadline afterward.
func clientIDOnConnForTest(t *testing.T, sr *SimpleRedis, pooled *pooledConn) string {
	t.Helper()
	values, reusable, err := sr.do(pooled, [][]byte{[]byte("CLIENT"), []byte("ID")})
	_ = pooled.netConn.SetDeadline(time.Time{})
	if err != nil {
		t.Fatalf("CLIENT ID: %v", err)
	}
	if !reusable || len(values) != 1 {
		t.Fatalf("CLIENT ID reusable=%v slots=%d", reusable, len(values))
	}
	return string(values[0])
}
