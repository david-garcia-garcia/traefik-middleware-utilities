package windowcounter

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
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

// liveEnvPrefix starts every live env of this package: WINDOWCOUNTER_LIVE_<ENGINE>.
const liveEnvPrefix = "WINDOWCOUNTER_LIVE"

// liveEngineAddrs skips under -short or when no WINDOWCOUNTER_LIVE_<ENGINE> is set, else returns the engines whose addrs are set.
func liveEngineAddrs(tb testing.TB) []liveEngineForTest {
	tb.Helper()
	if testing.Short() {
		tb.Skip("live engines skipped under -short")
	}
	engines, allUnset := lookupLiveEngineAddrs(os.Getenv, liveEnvPrefix, "")
	if allUnset {
		tb.Skip(liveEngineEnvList(liveEnvPrefix, "") + " unset")
	}
	return engines
}

// runForEachLiveEngine table-drives Redis, Dragonfly, then Valkey from WINDOWCOUNTER_LIVE_<ENGINE>.
func runForEachLiveEngine(t *testing.T, run func(t *testing.T, addr string)) {
	t.Helper()
	for _, backend := range liveEngineAddrs(t) {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			run(t, backend.addr)
		})
	}
}

// liveTTLScript returns TTL for KEYS[1] so live tests can assert first-hit EXPIRE landed.
const liveTTLScript = `return redis.call('TTL', KEYS[1])`

// waitLiveClient retries Incr until the engine accepts connections or the wait expires.
func waitLiveClient(tb testing.TB, addr string) *simpleredis.SimpleRedis {
	tb.Helper()
	client, err := simpleredis.New(simpleredis.Config{Host: addr})
	if err != nil {
		tb.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Incr(context.Background(), "windowcounter-live-probe")
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			tb.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestLive_Limiter proves Take, Peek, buffered share, and first-hit EXPIRE on live Redis, Dragonfly, and Valkey.
func TestLive_Limiter(t *testing.T) {
	runForEachLiveEngine(t, runLiveLimiterBackend)
}

// runLiveLimiterBackend runs exact N-then-deny, the denied-Take status, buffered share, sliding boundary, Peek, and
// expire-on-first-hit against one engine.
func runLiveLimiterBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	t.Run("deniedTakeStatus", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Real clock and a key unique to this run, so a second run on the same engine starts from an empty window.
		key := t.Name() + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
		const limit int64 = 3
		window := time.Minute
		for i := int64(0); i < limit; i++ {
			if _, takeErr := limiter.Take(context.Background(), key, limit, window); takeErr != nil {
				t.Fatal(takeErr)
			}
		}
		status, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed || status.Remaining != 0 {
			t.Fatalf("want deny with Remaining 0, got %+v", status)
		}
		if status.Reset <= 0 || status.Reset > window {
			t.Fatalf("Reset %v want in (0, %v]", status.Reset, window)
		}
		if status.RetryAfter <= 0 || status.RetryAfter > 2*window {
			t.Fatalf("RetryAfter %v want in (0, %v]", status.RetryAfter, 2*window)
		}
	})
	t.Run("exactNThenDeny", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 3
		window := time.Minute
		for i := int64(0); i < limit; i++ {
			status, takeErr := limiter.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !status.Allowed {
				t.Fatalf("take %d denied", i+1)
			}
		}
		status, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatal("want deny after N")
		}
	})
	t.Run("bufferedTwoClients", func(t *testing.T) {
		aClient := waitLiveClient(t, addr)
		bClient := waitLiveClient(t, addr)
		a, err := New(aClient, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		b, err := New(bClient, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 3
		window := time.Minute
		for i := 0; i < 2; i++ {
			status, takeErr := a.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !status.Allowed {
				t.Fatal("a denied early")
			}
			status, takeErr = b.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !status.Allowed {
				t.Fatal("b denied early")
			}
		}
		a.Sleep()
		b.Sleep()
		status, err := a.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatal("shared limit last-write-wins")
		}
		a.Close()
		b.Close()
	})
	t.Run("slidingBoundary", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		window := 10 * time.Second
		const limit int64 = 2
		start := time.Now().Unix() / 10 * 10
		now := time.Unix(start, 0).Add(9 * time.Second)
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := int64(0); i < limit; i++ {
			status, takeErr := limiter.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !status.Allowed {
				t.Fatalf("fill take %d denied", i+1)
			}
		}
		now = time.Unix(start, 0).Add(window)
		limiter.SetNowForTest(func() time.Time { return now })
		status, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatal("sliding boundary doubled like a fixed window")
		}
	})
	t.Run("peekThenTake", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 5
		window := time.Minute
		for i := 0; i < 3; i++ {
			status, peekErr := limiter.Peek(context.Background(), key, limit, window)
			if peekErr != nil {
				t.Fatal(peekErr)
			}
			if !status.Allowed {
				t.Fatalf("peek %d denied, estimated %v", i+1, status.Estimated)
			}
		}
		status, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Allowed {
			t.Fatal("take after peeks denied")
		}
		if status.Estimated != 1 {
			t.Fatalf("take estimated %v want 1", status.Estimated)
		}
	})
	t.Run("peekDeniedThenSlides", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		window := 10 * time.Second
		const limit int64 = 2
		start := time.Unix(1_700_000_000, 0)
		now := start
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := int64(0); i < limit+1; i++ {
			if _, takeErr := limiter.Take(context.Background(), key, limit, window); takeErr != nil {
				t.Fatal(takeErr)
			}
		}
		status, err := limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatalf("peek after fill allowed, estimated %v", status.Estimated)
		}
		now = start.Add(window)
		limiter.SetNowForTest(func() time.Time { return now })
		status, err = limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatalf("peek at next window start allowed, estimated %v (weight still 1)", status.Estimated)
		}
		now = start.Add(window + 4*time.Second)
		limiter.SetNowForTest(func() time.Time { return now })
		status, err = limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Allowed {
			t.Fatalf("peek after formula cooldown denied, estimated %v", status.Estimated)
		}
		if status.Estimated > float64(limit) {
			t.Fatalf("allowed estimate %v want <= %d", status.Estimated, limit)
		}
	})
	t.Run("bufferedPeek", func(t *testing.T) {
		limiter, err := New(client, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { limiter.Close() })
		now := time.Unix(1_700_000_000, 0)
		limiter.SetNowForTest(func() time.Time { return now })
		const limit int64 = 5
		window := time.Minute
		key := t.Name()
		for i := 0; i < 5; i++ {
			status, peekErr := limiter.Peek(context.Background(), key, limit, window)
			if peekErr != nil {
				t.Fatal(peekErr)
			}
			if !status.Allowed {
				t.Fatalf("peek %d denied, estimated %v", i+1, status.Estimated)
			}
			if status.Estimated != 0 {
				t.Fatalf("peek %d estimated %v want 0", i+1, status.Estimated)
			}
		}
		status, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Allowed {
			t.Fatal("take after buffered peeks denied")
		}
		if status.Estimated != 1 {
			t.Fatalf("take estimated %v want 1", status.Estimated)
		}
	})
	t.Run("expireOnFirstHit", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		window := 10 * time.Second
		now := time.Unix(1_700_000_000, 0)
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		if _, err := limiter.Take(context.Background(), key, 5, window); err != nil {
			t.Fatal(err)
		}
		windowStart := now.Unix() / 10 * 10
		redisKey := redisWindowKey(key, windowStart)
		values, err := client.Eval(context.Background(), liveTTLScript, simpleredis.ScriptSHA1Hex(liveTTLScript), []string{redisKey}, nil)
		if err != nil {
			t.Fatalf("TTL Eval: %v", err)
		}
		if len(values) != 1 {
			t.Fatalf("TTL slots %d, want 1", len(values))
		}
		ttl, err := strconv.ParseInt(string(values[0]), 10, 64)
		if err != nil {
			t.Fatalf("TTL parse %q: %v", values[0], err)
		}
		if ttl <= 0 {
			t.Fatalf("TTL = %d, want > 0 after first hit EXPIRE", ttl)
		}
	})
}
