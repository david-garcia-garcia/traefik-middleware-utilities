package tokenbucket

import (
	"os"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// liveEngineForTest is one Redis or Dragonfly address taken from LIVE env.
type liveEngineForTest struct {
	name string
	addr string
}

// liveEngineAddrs skips under -short or both env unset, fails when exactly one addr is set, else returns both engines.
func liveEngineAddrs(t *testing.T, redisEnv, dragonflyEnv string) []liveEngineForTest {
	t.Helper()
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	redisAddr := os.Getenv(redisEnv)
	dragonflyAddr := os.Getenv(dragonflyEnv)
	if redisAddr == "" && dragonflyAddr == "" {
		t.Skip(redisEnv + " and " + dragonflyEnv + " unset")
	}
	if redisAddr == "" || dragonflyAddr == "" {
		t.Fatal("exactly one live engine address is set; set both or neither")
	}
	return []liveEngineForTest{
		{name: "redis", addr: redisAddr},
		{name: "dragonfly", addr: dragonflyAddr},
	}
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

// TestLive_Limiter proves burst, two-instance share, memory agreement, and refund on live Redis and Dragonfly.
func TestLive_Limiter(t *testing.T) {
	runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", runLiveBackend)
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
			allowed, _, allowErr := limiter.Allow(key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !allowed {
				t.Fatalf("burst %d denied", i+1)
			}
		}
		allowed, wait, err := limiter.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed || wait <= 0 {
			t.Fatalf("want delayed admit, allowed %v wait %v", allowed, wait)
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
			allowed, _, allowErr := a.Allow(key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !allowed {
				t.Fatalf("a burst %d denied", i+1)
			}
		}
		allowed, _, err := b.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
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
			mAllowed, mWait, mErr := mem.Allow(key)
			if mErr != nil {
				t.Fatal(mErr)
			}
			rAllowed, rWait, rErr := red.Allow(key)
			if rErr != nil {
				t.Fatal(rErr)
			}
			if mAllowed != rAllowed {
				t.Fatalf("step %d allowed mem %v redis %v", i, mAllowed, rAllowed)
			}
			if waitClass(mWait) != waitClass(rWait) {
				t.Fatalf("step %d wait mem %v redis %v", i, mWait, rWait)
			}
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
			allowed, _, allowErr := limiter.Allow(key)
			if allowErr != nil {
				t.Fatal(allowErr)
			}
			if !allowed {
				t.Fatalf("burst %d denied", i+1)
			}
		}
		allowed, wait, err := limiter.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if allowed || wait <= time.Microsecond {
			t.Fatalf("want deny wait>maxDelay, got allowed %v wait %v", allowed, wait)
		}
		allowedAfterRefund, waitAfterRefund, err := limiter.Allow(key)
		if err != nil {
			t.Fatal(err)
		}
		if allowedAfterRefund {
			t.Fatal("refund must not admit a stacked consume")
		}
		if waitAfterRefund != wait {
			t.Fatalf("wait after refund %v want %v (not stacked)", waitAfterRefund, wait)
		}
	})
}

// waitLiveClient retries Eval until the engine accepts connections or the wait expires.
func waitLiveClient(t *testing.T, addr string) *simpleredis.SimpleRedis {
	t.Helper()
	client := simpleredis.New(simpleredis.Config{Host: addr})
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := client.Eval("return 1", nil, nil)
		if err == nil {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
