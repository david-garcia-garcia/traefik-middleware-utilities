package simpleredis

import (
	"bufio"
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// liveEngineForTest is one live engine address taken from LIVE env.
type liveEngineForTest struct {
	name string
	addr string
}

// liveEngineNames are the engines a live test runs against, in run order.
var liveEngineNames = []string{"redis", "dragonfly", "valkey"}

// liveEngineEnv is the env var holding name's address: prefix, an underscore, the upper-cased name, then suffix.
func liveEngineEnv(prefix, name, suffix string) string {
	return prefix + "_" + strings.ToUpper(name) + suffix
}

// lookupLiveEngineAddrs returns the engines whose env is set, in liveEngineNames order, or allUnset when none is.
func lookupLiveEngineAddrs(getenv func(string) string, prefix, suffix string) (engines []liveEngineForTest, allUnset bool) {
	for _, name := range liveEngineNames {
		if addr := getenv(liveEngineEnv(prefix, name, suffix)); addr != "" {
			engines = append(engines, liveEngineForTest{name: name, addr: addr})
		}
	}
	return engines, len(engines) == 0
}

// liveEngineEnvList is every engine env for prefix and suffix, comma-separated, for skip messages.
func liveEngineEnvList(prefix, suffix string) string {
	names := make([]string, 0, len(liveEngineNames))
	for _, name := range liveEngineNames {
		names = append(names, liveEngineEnv(prefix, name, suffix))
	}
	return strings.Join(names, ", ")
}

// TestLookupLiveEngineAddrs proves skip-all and one-or-more engines in run order without dialing (runs under -short).
func TestLookupLiveEngineAddrs(t *testing.T) {
	const prefix = "PKG_LIVE"
	lookup := func(env map[string]string, suffix string) ([]liveEngineForTest, bool) {
		return lookupLiveEngineAddrs(func(key string) string { return env[key] }, prefix, suffix)
	}
	t.Run("allUnset", func(t *testing.T) {
		engines, allUnset := lookup(nil, "")
		if !allUnset || len(engines) != 0 {
			t.Fatalf("lookup(none) = %v unset=%v", engines, allUnset)
		}
	})
	for _, name := range liveEngineNames {
		name := name
		t.Run("only-"+name, func(t *testing.T) {
			engines, allUnset := lookup(map[string]string{liveEngineEnv(prefix, name, ""): "127.0.0.1:7000"}, "")
			if allUnset || len(engines) != 1 || engines[0].name != name || engines[0].addr != "127.0.0.1:7000" {
				t.Fatalf("only %s = %+v unset=%v", name, engines, allUnset)
			}
		})
	}
	t.Run("allSetInRunOrder", func(t *testing.T) {
		env := map[string]string{
			"PKG_LIVE_VALKEY":    "127.0.0.1:6385",
			"PKG_LIVE_DRAGONFLY": "127.0.0.1:6380",
			"PKG_LIVE_REDIS":     "127.0.0.1:6379",
		}
		engines, allUnset := lookup(env, "")
		if allUnset || len(engines) != 3 {
			t.Fatalf("allSet = %v unset=%v", engines, allUnset)
		}
		want := []liveEngineForTest{{"redis", "127.0.0.1:6379"}, {"dragonfly", "127.0.0.1:6380"}, {"valkey", "127.0.0.1:6385"}}
		for i := range want {
			if engines[i] != want[i] {
				t.Fatalf("engines[%d] = %+v, want %+v", i, engines[i], want[i])
			}
		}
	})
	t.Run("suffixSelectsItsOwnEnv", func(t *testing.T) {
		env := map[string]string{"PKG_LIVE_REDIS": "127.0.0.1:6379", "PKG_LIVE_VALKEY_AUTH": "127.0.0.1:6386"}
		engines, allUnset := lookup(env, "_AUTH")
		if allUnset || len(engines) != 1 || engines[0].name != "valkey" || engines[0].addr != "127.0.0.1:6386" {
			t.Fatalf("suffix _AUTH = %+v unset=%v, want valkey 127.0.0.1:6386", engines, allUnset)
		}
	})
	t.Run("envList", func(t *testing.T) {
		got := liveEngineEnvList(prefix, "_TLS")
		want := "PKG_LIVE_REDIS_TLS, PKG_LIVE_DRAGONFLY_TLS, PKG_LIVE_VALKEY_TLS"
		if got != want {
			t.Fatalf("liveEngineEnvList = %q, want %q", got, want)
		}
	})
}

// liveEnvPrefix starts every simpleredis live env: SIMPLEREDIS_LIVE_<ENGINE> plus an optional suffix such as _AUTH or _TLS.
const liveEnvPrefix = "SIMPLEREDIS_LIVE"

// liveEngineAddrs skips under -short or when no engine env with suffix is set, else returns the engines whose addrs are set.
func liveEngineAddrs(tb testing.TB, suffix string) []liveEngineForTest {
	tb.Helper()
	if testing.Short() {
		tb.Skip("live engines skipped under -short")
	}
	engines, allUnset := lookupLiveEngineAddrs(os.Getenv, liveEnvPrefix, suffix)
	if allUnset {
		tb.Skip(liveEngineEnvList(liveEnvPrefix, suffix) + " unset")
	}
	return engines
}

// runForEachLiveEngine table-drives Redis, Dragonfly, then Valkey from SIMPLEREDIS_LIVE_<ENGINE><suffix>.
func runForEachLiveEngine(t *testing.T, suffix string, run func(t *testing.T, addr string)) {
	t.Helper()
	for _, backend := range liveEngineAddrs(t, suffix) {
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
	values, err := client.Eval(context.Background(), ttlScript, ScriptSHA1Hex(ttlScript), []string{key}, nil)
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
	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, PoolTimeout: 80 * time.Millisecond})
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := client.Set(context.Background(), "simpleredis-live-probe", []byte("1"), 60); err == nil {
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
	if err := netConn.SetDeadline(time.Now().Add(defaultCommandTimeout)); err != nil {
		t.Fatal(err)
	}
	killConn := &pooledConn{netConn: netConn, writer: bufio.NewWriter(netConn)}
	reader := bufio.NewReader(netConn)
	if err := writeCommand(killConn, [][]byte{[]byte("CLIENT"), []byte("KILL"), []byte(filter), []byte(value)}); err != nil {
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
	values, reusable, err := sr.do(context.Background(), pooled, [][]byte{[]byte("CLIENT"), []byte("ID")})
	_ = pooled.netConn.SetDeadline(time.Time{})
	if err != nil {
		t.Fatalf("CLIENT ID: %v", err)
	}
	if !reusable || len(values) != 1 {
		t.Fatalf("CLIENT ID reusable=%v slots=%d", reusable, len(values))
	}
	return string(values[0])
}
