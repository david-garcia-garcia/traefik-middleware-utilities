package tokenbucket

import (
	"context"
	"os"
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

// liveEnvPrefix starts every live env of this package: TOKENBUCKET_LIVE_<ENGINE>.
const liveEnvPrefix = "TOKENBUCKET_LIVE"

// liveEngineAddrs skips under -short or when no TOKENBUCKET_LIVE_<ENGINE> is set, else returns the engines whose addrs are set.
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

// runForEachLiveEngine table-drives Redis, Dragonfly, then Valkey from TOKENBUCKET_LIVE_<ENGINE>.
func runForEachLiveEngine(t *testing.T, run func(t *testing.T, addr string)) {
	t.Helper()
	for _, backend := range liveEngineAddrs(t) {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			run(t, backend.addr)
		})
	}
}

// TestLive_Limiter proves burst, two-instance share, memory agreement, and refund on live Redis, Dragonfly, and Valkey.
func TestLive_Limiter(t *testing.T) {
	runForEachLiveEngine(t, runLiveBackend)
}

// runLiveBackend runs burst, two-instance share, memory/Redis agreement, and refund past maxDelay against one engine.
func runLiveBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	now := time.Unix(time.Now().Unix(), 0)
	t.Run("burstAfterIdle", func(t *testing.T) {
		limiter, err := NewRedis(client, 1, 3, time.Hour, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 3; i++ {
			status, allowErr := limiter.Allow(context.Background(), key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !status.Allowed {
				t.Fatalf("burst %d denied", i+1)
			}
		}
		status, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Allowed || (status.Delay+status.RetryAfter) <= 0 {
			t.Fatalf("want delayed admit, allowed %v wait %v", status.Allowed, status.Delay+status.RetryAfter)
		}
	})
	t.Run("twoInstancesShare", func(t *testing.T) {
		aClient := waitLiveClient(t, addr)
		bClient := waitLiveClient(t, addr)
		a, err := NewRedis(aClient, 1, 3, time.Microsecond, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NewRedis(bClient, 1, 3, time.Microsecond, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		a.SetNowForTest(func() time.Time { return now })
		b.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 3; i++ {
			status, allowErr := a.Allow(context.Background(), key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !status.Allowed {
				t.Fatalf("a burst %d denied", i+1)
			}
		}
		status, err := b.Allow(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed {
			t.Fatal("second instance granted another burst")
		}
	})
	t.Run("memoryAgrees", func(t *testing.T) {
		mem, err := NewMemory(2, 2, agreeMaxDelay, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		red, err := NewRedis(client, 2, 2, agreeMaxDelay, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		mem.SetNowForTest(func() time.Time { return now })
		red.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 4; i++ {
			mStatus, mErr := mem.Allow(context.Background(), key)
			if mErr != nil {
				t.Fatal(mErr)
			}
			rStatus, rErr := red.Allow(context.Background(), key)
			if rErr != nil {
				t.Fatal(rErr)
			}
			assertStatusAgrees(t, i, mStatus, rStatus)
		}
	})
	t.Run("refundPastMaxDelay", func(t *testing.T) {
		limiter, err := NewRedis(client, 1, 3, time.Microsecond, testTTL)
		if err != nil {
			t.Fatal(err)
		}
		limiter.SetNowForTest(func() time.Time { return now })
		key := t.Name()
		for i := 0; i < 3; i++ {
			status, allowErr := limiter.Allow(context.Background(), key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !status.Allowed {
				t.Fatalf("burst %d denied", i+1)
			}
		}
		status, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if status.Allowed || (status.Delay+status.RetryAfter) <= time.Microsecond {
			t.Fatalf("want deny wait>maxDelay, got allowed %v wait %v", status.Allowed, status.Delay+status.RetryAfter)
		}
		statusAfterRefund, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if statusAfterRefund.Allowed {
			t.Fatal("refund must not admit a stacked consume")
		}
		if (statusAfterRefund.Delay + statusAfterRefund.RetryAfter) != (status.Delay + status.RetryAfter) {
			t.Fatalf("wait after refund %v want %v (not stacked)", statusAfterRefund.Delay+statusAfterRefund.RetryAfter, status.Delay+status.RetryAfter)
		}
	})
}

// waitLiveClient retries Eval until the engine accepts connections or the wait expires.
func waitLiveClient(tb testing.TB, addr string) *simpleredis.SimpleRedis {
	tb.Helper()
	client, err := simpleredis.New(simpleredis.Config{Host: addr})
	if err != nil {
		tb.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Eval(context.Background(), "return 1", simpleredis.ScriptSHA1Hex("return 1"), nil, nil)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			tb.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
