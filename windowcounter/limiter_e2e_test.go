package windowcounter

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// liveEngineForTest is one Redis or Dragonfly address taken from LIVE env.
type liveEngineForTest struct {
	name string
	addr string
}

// lookupLiveEngineAddrs returns the engines whose addresses are set, or bothUnset when neither is set.
func lookupLiveEngineAddrs(redisAddr, dragonflyAddr string) (engines []liveEngineForTest, bothUnset bool) {
	if redisAddr == "" && dragonflyAddr == "" {
		return nil, true
	}
	if redisAddr != "" {
		engines = append(engines, liveEngineForTest{name: "redis", addr: redisAddr})
	}
	if dragonflyAddr != "" {
		engines = append(engines, liveEngineForTest{name: "dragonfly", addr: dragonflyAddr})
	}
	return engines, false
}

// liveEngineAddrs skips under -short or both env unset, else returns the engines whose addrs are set.
func liveEngineAddrs(t *testing.T, redisEnv, dragonflyEnv string) []liveEngineForTest {
	t.Helper()
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	engines, bothUnset := lookupLiveEngineAddrs(os.Getenv(redisEnv), os.Getenv(dragonflyEnv))
	if bothUnset {
		t.Skip(redisEnv + " and " + dragonflyEnv + " unset")
	}
	return engines
}

// TestLookupLiveEngineAddrs proves skip-both and one-or-both engines without dialing (runs under -short).
func TestLookupLiveEngineAddrs(t *testing.T) {
	t.Run("bothUnset", func(t *testing.T) {
		engines, bothUnset := lookupLiveEngineAddrs("", "")
		if !bothUnset || len(engines) != 0 {
			t.Fatalf("lookup(\"\",\"\") = %v unset=%v", engines, bothUnset)
		}
	})
	t.Run("onlyRedis", func(t *testing.T) {
		engines, bothUnset := lookupLiveEngineAddrs("127.0.0.1:6379", "")
		if bothUnset || len(engines) != 1 || engines[0].name != "redis" || engines[0].addr != "127.0.0.1:6379" {
			t.Fatalf("onlyRedis = %+v unset=%v, want redis 127.0.0.1:6379", engines, bothUnset)
		}
	})
	t.Run("onlyDragonfly", func(t *testing.T) {
		engines, bothUnset := lookupLiveEngineAddrs("", "127.0.0.1:6380")
		if bothUnset || len(engines) != 1 || engines[0].name != "dragonfly" || engines[0].addr != "127.0.0.1:6380" {
			t.Fatalf("onlyDragonfly = %+v unset=%v, want dragonfly 127.0.0.1:6380", engines, bothUnset)
		}
	})
	t.Run("bothSet", func(t *testing.T) {
		engines, bothUnset := lookupLiveEngineAddrs("127.0.0.1:6379", "127.0.0.1:6380")
		if bothUnset || len(engines) != 2 {
			t.Fatalf("bothSet = %v unset=%v", engines, bothUnset)
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

// liveTTLScript returns TTL for KEYS[1] so live tests can assert first-hit EXPIRE landed.
const liveTTLScript = `return redis.call('TTL', KEYS[1])`

// waitLiveClient retries Incr until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *simpleredis.SimpleRedis {
	t.Helper()
	client := simpleredis.New(simpleredis.Config{Host: addr})
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Incr(context.Background(), "windowcounter-live-probe")
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestLive_Limiter proves Take, Peek, buffered share, and first-hit EXPIRE on live Redis and Dragonfly.
func TestLive_Limiter(t *testing.T) {
	runForEachLiveEngine(t, "WINDOWCOUNTER_LIVE_REDIS", "WINDOWCOUNTER_LIVE_DRAGONFLY", runLiveLimiterBackend)
}

// runLiveLimiterBackend runs exact N-then-deny, buffered share, sliding boundary, Peek, and expire-on-first-hit against one engine.
func runLiveLimiterBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveClient(t, addr)
	t.Run("exactNThenDeny", func(t *testing.T) {
		limiter, err := New(client, 0)
		if err != nil {
			t.Fatal(err)
		}
		key := t.Name()
		const limit int64 = 3
		window := time.Minute
		for i := int64(0); i < limit; i++ {
			allowed, _, takeErr := limiter.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatalf("take %d denied", i+1)
			}
		}
		allowed, _, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
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
			allowed, _, takeErr := a.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatal("a denied early")
			}
			allowed, _, takeErr = b.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatal("b denied early")
			}
		}
		a.Sleep()
		b.Sleep()
		allowed, _, err := a.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
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
			allowed, _, takeErr := limiter.Take(context.Background(), key, limit, window)
			if takeErr != nil {
				t.Fatal(takeErr)
			}
			if !allowed {
				t.Fatalf("fill take %d denied", i+1)
			}
		}
		now = time.Unix(start, 0).Add(window)
		limiter.SetNowForTest(func() time.Time { return now })
		allowed, _, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
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
			allowed, estimated, peekErr := limiter.Peek(context.Background(), key, limit, window)
			if peekErr != nil {
				t.Fatal(peekErr)
			}
			if !allowed {
				t.Fatalf("peek %d denied, estimated %v", i+1, estimated)
			}
		}
		allowed, estimated, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatal("take after peeks denied")
		}
		if estimated != 1 {
			t.Fatalf("take estimated %v want 1", estimated)
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
			if _, _, takeErr := limiter.Take(context.Background(), key, limit, window); takeErr != nil {
				t.Fatal(takeErr)
			}
		}
		allowed, estimated, err := limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatalf("peek after fill allowed, estimated %v", estimated)
		}
		now = start.Add(window)
		limiter.SetNowForTest(func() time.Time { return now })
		allowed, estimated, err = limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatalf("peek at next window start allowed, estimated %v (weight still 1)", estimated)
		}
		now = start.Add(window + 4*time.Second)
		limiter.SetNowForTest(func() time.Time { return now })
		allowed, estimated, err = limiter.Peek(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatalf("peek after formula cooldown denied, estimated %v", estimated)
		}
		if estimated > float64(limit) {
			t.Fatalf("allowed estimate %v want <= %d", estimated, limit)
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
			allowed, estimated, peekErr := limiter.Peek(context.Background(), key, limit, window)
			if peekErr != nil {
				t.Fatal(peekErr)
			}
			if !allowed {
				t.Fatalf("peek %d denied, estimated %v", i+1, estimated)
			}
			if estimated != 0 {
				t.Fatalf("peek %d estimated %v want 0", i+1, estimated)
			}
		}
		allowed, estimated, err := limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatal("take after buffered peeks denied")
		}
		if estimated != 1 {
			t.Fatalf("take estimated %v want 1", estimated)
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
		if _, _, err := limiter.Take(context.Background(), key, 5, window); err != nil {
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
