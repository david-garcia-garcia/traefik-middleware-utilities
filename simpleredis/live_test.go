package simpleredis

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

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

// runLivePeerCloseRecovery Gets, CLIENT KILLs that pooled ADDR (or ID), then Gets on a new dial.
func runLivePeerCloseRecovery(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedisForTest(t, addr)
	t.Cleanup(func() { client.Close() })

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

	client.mu.Lock()
	if len(client.idle) != 1 {
		client.mu.Unlock()
		t.Fatalf("idle after Get = %d, want 1", len(client.idle))
	}
	killed := client.idle[0]
	client.mu.Unlock()

	killPooledIdleAddrOrIDForTest(t, client)

	got, err = client.Get(key)
	if err != nil {
		t.Fatalf("Get after CLIENT KILL: %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("Get after CLIENT KILL = %q, want ok", got)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	for _, conn := range client.idle {
		if conn == killed {
			t.Fatal("killed socket was reused")
		}
	}
}

// waitLiveSimpleRedisForTest retries Set until the engine accepts connections or the wait expires.
func waitLiveSimpleRedisForTest(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := &SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		err := client.Set("simpleredis-live-probe", []byte("ok"), 60)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// killPooledIdleAddrOrIDForTest sends CLIENT KILL ADDR of the idle pooled socket from a sidecar.
// If the engine reports 0 (Docker port-publish NAT), it KILLs by that socket's CLIENT ID instead.
func killPooledIdleAddrOrIDForTest(t *testing.T, sr *SimpleRedis) {
	t.Helper()
	sr.mu.Lock()
	if len(sr.idle) == 0 {
		sr.mu.Unlock()
		t.Fatal("no idle pooled conn to kill")
	}
	pooled := sr.idle[0]
	addr := pooled.netConn.LocalAddr().String()
	sr.mu.Unlock()

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
	netConn, err := net.DialTimeout("tcp", host, dialTimeout)
	if err != nil {
		t.Fatalf("sidecar dial %s: %v", host, err)
	}
	defer netConn.Close()
	if err := netConn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
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
