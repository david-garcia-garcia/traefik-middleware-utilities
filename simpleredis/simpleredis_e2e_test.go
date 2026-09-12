package simpleredis

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// liveEngineForTest is one Redis or Dragonfly address taken from LIVE env.
type liveEngineForTest struct {
	name string
	addr string
}

// errLiveEngineOneAddr is the fail-closed result when exactly one of Redis or Dragonfly is set.
var errLiveEngineOneAddr = errors.New("exactly one live engine address is set; set both or neither")

// lookupLiveEngineAddrs returns both engines, bothUnset when neither addr is set, or errLiveEngineOneAddr when exactly one is set.
func lookupLiveEngineAddrs(redisAddr, dragonflyAddr string) (engines []liveEngineForTest, bothUnset bool, err error) {
	if redisAddr == "" && dragonflyAddr == "" {
		return nil, true, nil
	}
	if redisAddr == "" || dragonflyAddr == "" {
		return nil, false, errLiveEngineOneAddr
	}
	return []liveEngineForTest{
		{name: "redis", addr: redisAddr},
		{name: "dragonfly", addr: dragonflyAddr},
	}, false, nil
}

// liveEngineAddrs skips under -short or both env unset, fails when exactly one addr is set, else returns both engines.
func liveEngineAddrs(t *testing.T, redisEnv, dragonflyEnv string) []liveEngineForTest {
	t.Helper()
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	engines, bothUnset, err := lookupLiveEngineAddrs(os.Getenv(redisEnv), os.Getenv(dragonflyEnv))
	if bothUnset {
		t.Skip(redisEnv + " and " + dragonflyEnv + " unset")
	}
	if err != nil {
		t.Fatal(err)
	}
	return engines
}

// TestLookupLiveEngineAddrs proves both-or-neither without dialing an engine (runs under -short).
func TestLookupLiveEngineAddrs(t *testing.T) {
	t.Run("bothUnset", func(t *testing.T) {
		engines, bothUnset, err := lookupLiveEngineAddrs("", "")
		if err != nil || !bothUnset || len(engines) != 0 {
			t.Fatalf("lookup(\"\",\"\") = %v unset=%v err=%v", engines, bothUnset, err)
		}
	})
	t.Run("onlyRedis", func(t *testing.T) {
		_, bothUnset, err := lookupLiveEngineAddrs("127.0.0.1:6379", "")
		if bothUnset || !errors.Is(err, errLiveEngineOneAddr) {
			t.Fatalf("onlyRedis unset=%v err=%v, want errLiveEngineOneAddr", bothUnset, err)
		}
	})
	t.Run("onlyDragonfly", func(t *testing.T) {
		_, bothUnset, err := lookupLiveEngineAddrs("", "127.0.0.1:6380")
		if bothUnset || !errors.Is(err, errLiveEngineOneAddr) {
			t.Fatalf("onlyDragonfly unset=%v err=%v, want errLiveEngineOneAddr", bothUnset, err)
		}
	})
	t.Run("bothSet", func(t *testing.T) {
		engines, bothUnset, err := lookupLiveEngineAddrs("127.0.0.1:6379", "127.0.0.1:6380")
		if err != nil || bothUnset || len(engines) != 2 {
			t.Fatalf("bothSet = %v unset=%v err=%v", engines, bothUnset, err)
		}
		if engines[0].name != "redis" || engines[0].addr != "127.0.0.1:6379" {
			t.Fatalf("engines[0] = %+v, want redis 127.0.0.1:6379", engines[0])
		}
		if engines[1].name != "dragonfly" || engines[1].addr != "127.0.0.1:6380" {
			t.Fatalf("engines[1] = %+v, want dragonfly 127.0.0.1:6380", engines[1])
		}
	})
}

// runForEachLiveEngine table-drives Redis then Dragonfly after liveEngineAddrs.
func runForEachLiveEngine(t *testing.T, redisEnv, dragonflyEnv string, run func(t *testing.T, addr string)) {
	t.Helper()
	for _, backend := range liveEngineAddrs(t, redisEnv, dragonflyEnv) {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			run(t, backend.addr)
		})
	}
}

// ttlScript returns TTL for KEYS[1] so live tests can assert expiry landed.
const ttlScript = `return redis.call('TTL', KEYS[1])`

// assertLiveTTLPositive Evals TTL for key and fails unless the remaining seconds are positive.
func assertLiveTTLPositive(t *testing.T, client *SimpleRedis, key string) {
	t.Helper()
	values, err := client.Eval(ttlScript, ScriptSHA1Hex(ttlScript), []string{key}, nil)
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
