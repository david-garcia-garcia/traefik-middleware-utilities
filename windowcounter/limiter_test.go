package windowcounter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// newSimpleRedisForTest returns a New client for tests.
func newSimpleRedisForTest(t testing.TB, host string) *simpleredis.SimpleRedis {
	t.Helper()
	return simpleredis.New(simpleredis.Config{Host: host})
}

func TestCountFromRedisGet_WrappedMissIsZero(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", simpleredis.ErrMiss)
	if wrapped.Error() == simpleredis.RedisMiss {
		t.Fatal("wrapped.Error() still equals RedisMiss; test would not catch dest string compare")
	}
	n, err := countFromRedisGet(nil, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("count %d, want 0", n)
	}
}

func TestTake_NThenDeny(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	const limit int64 = 3
	window := time.Minute
	for i := int64(0); i < limit; i++ {
		allowed, estimated, takeErr := limiter.Take(context.Background(), "k", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("take %d denied, estimated %v", i+1, estimated)
		}
	}
	allowed, estimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("take %d allowed, estimated %v", limit+1, estimated)
	}
	if estimated <= float64(limit) {
		t.Fatalf("denied estimate %v want > %d", estimated, limit)
	}
}

func TestTake_ExpireOnFirstHit(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Second
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take(context.Background(), "k", 5, window); err != nil {
		t.Fatal(err)
	}
	got := fake.lastExpireCommand()
	if len(got) < 3 || got[0] != "EXPIRE" || got[2] != "20" {
		t.Fatalf("expire argv %v want EXPIRE … 20", got)
	}
}

func TestTake_SlidingBoundaryDoesNotDouble(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Second
	const limit int64 = 2
	start := time.Unix(1_700_000_000, 0)
	now := start.Add(9 * time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := int64(0); i < limit; i++ {
		allowed, _, takeErr := limiter.Take(context.Background(), "k", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("fill take %d denied", i+1)
		}
	}
	now = start.Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, estimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("boundary take allowed, estimated %v (fixed-window double)", estimated)
	}
}

func TestTake_Unreachable(t *testing.T) {
	client := newSimpleRedisForTest(t, "127.0.0.1:1")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Take(context.Background(), "k", 1, time.Minute)
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("err %v want %s", err, simpleredis.RedisUnreachable)
	}
}

func TestNew_NegativeSyncRate(t *testing.T) {
	client := simpleredis.New(simpleredis.Config{})
	if _, err := New(client, -time.Second); err == nil {
		t.Fatal("want error")
	}
}

func TestNew_FloorsSyncRateBelow20ms(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	if limiter.syncRate != minSyncRate {
		t.Fatalf("syncRate %v want %v", limiter.syncRate, minSyncRate)
	}
}

func TestTake_SubSecondWindow(t *testing.T) {
	client := newSimpleRedisForTest(t, "127.0.0.1:1")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Take(context.Background(), "k", 1, 500*time.Millisecond)
	if err == nil {
		t.Fatal("want window error")
	}
}

func TestBuffered_TickerFlushesWithoutSleep(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, err := limiter.Take(context.Background(), "tick-k", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first take denied")
	}
	windowStart := now.Unix() / 60 * 60
	redisKey := redisWindowKey("tick-k", windowStart)
	deadline := time.Now().Add(time.Second)
	for {
		raw, getErr := client.Get(context.Background(), redisKey)
		if getErr == nil && string(raw) == "1" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("ticker did not flush; get err %v value %q", getErr, raw)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestBuffered_TwoClientsShareWithoutLastWriteWins(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := newSimpleRedisForTest(t, addr)
	bClient := newSimpleRedisForTest(t, addr)
	a, err := New(aClient, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(bClient, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	const limit int64 = 3
	window := time.Minute
	for i := 0; i < 2; i++ {
		allowed, _, takeErr := a.Take(context.Background(), "share", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("a take %d denied", i+1)
		}
		allowed, _, takeErr = b.Take(context.Background(), "share", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("b take %d denied", i+1)
		}
	}
	a.Sleep()
	b.Sleep()
	allowed, estimated, err := a.Take(context.Background(), "share", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("shared count last-write-wins? allowed estimated %v", estimated)
	}
}

func TestClose_StopsTickerAndKeepsRedis(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	limiter.Sleep()
	limiter.Close()
	if _, err := client.Incr(context.Background(), "still-open"); err != nil {
		t.Fatalf("redis closed by limiter: %v", err)
	}
	limiter.Wake()
	limiter.mu.Lock()
	running := limiter.stop != nil
	limiter.mu.Unlock()
	if running {
		t.Fatal("Wake after Close started a ticker")
	}
}

// TestWake_StartsTickerAfterSleep checks that Wake after a completed Sleep starts the flush ticker again, including a second Sleep while already stopped.
func TestWake_StartsTickerAfterSleep(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	defer limiter.Close()
	limiter.Sleep()
	limiter.Sleep()
	limiter.mu.Lock()
	stopped := limiter.stop == nil
	limiter.mu.Unlock()
	if !stopped {
		t.Fatal("Sleep left a ticker running")
	}
	limiter.Wake()
	limiter.mu.Lock()
	running := limiter.stop != nil
	limiter.mu.Unlock()
	if !running {
		t.Fatal("Wake after Sleep did not start a ticker")
	}
}

func TestAllow_IsTake(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	allowed, _, err := limiter.Allow(context.Background(), "k", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first Allow denied")
	}
}

func TestPeek_DoesNotIncrement(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 5
	window := time.Minute
	for i := 0; i < 5; i++ {
		allowed, estimated, peekErr := limiter.Peek(context.Background(), "k", limit, window)
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
	allowed, estimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("take after peeks denied")
	}
	if estimated != 1 {
		t.Fatalf("take estimated %v want 1", estimated)
	}
}

func TestPeek_BufferedDoesNotIncrement(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 5
	window := time.Minute
	for i := 0; i < 5; i++ {
		allowed, estimated, peekErr := limiter.Peek(context.Background(), "k", limit, window)
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
	allowed, estimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("take after buffered peeks denied")
	}
	if estimated != 1 {
		t.Fatalf("take estimated %v want 1", estimated)
	}
}

func TestPeek_AgreesWithTakeBeforeIncrement(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 3
	window := time.Minute
	for i := int64(0); i < limit-1; i++ {
		if _, _, takeErr := limiter.Take(context.Background(), "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	peekAllowed, peekEstimated, err := limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	takeAllowed, takeEstimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if takeAllowed != peekAllowed {
		t.Fatalf("take allowed %v peek allowed %v", takeAllowed, peekAllowed)
	}
	if takeEstimated != peekEstimated+1 {
		t.Fatalf("take estimated %v peek estimated %v", takeEstimated, peekEstimated)
	}
}

func TestPeek_BufferedTakeThenPeekDenies(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 2
	window := time.Minute
	for i := int64(0); i < limit+1; i++ {
		if _, _, takeErr := limiter.Take(context.Background(), "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	peekAllowed, peekEstimated, err := limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if peekAllowed {
		t.Fatalf("buffered peek after fill allowed, estimated %v", peekEstimated)
	}
	takeAllowed, takeEstimated, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if takeAllowed != peekAllowed {
		t.Fatalf("take allowed %v peek allowed %v", takeAllowed, peekAllowed)
	}
	if takeEstimated != peekEstimated+1 {
		t.Fatalf("take estimated %v peek estimated %v", takeEstimated, peekEstimated)
	}
}

func TestPeek_StaysDeniedThenSlidesAllowed(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Second
	const limit int64 = 2
	start := time.Unix(1_700_000_000, 0)
	now := start
	limiter.SetNowForTest(func() time.Time { return now })
	for i := int64(0); i < limit+1; i++ {
		if _, _, takeErr := limiter.Take(context.Background(), "k", limit, window); takeErr != nil {
			t.Fatal(takeErr)
		}
	}
	allowed, estimated, err := limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("peek after fill allowed, estimated %v", estimated)
	}
	now = start.Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, estimated, err = limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("peek at next window start allowed, estimated %v (weight still 1)", estimated)
	}
	now = start.Add(window + 4*time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, estimated, err = limiter.Peek(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatalf("peek after formula cooldown denied, estimated %v", estimated)
	}
	if estimated > float64(limit) {
		t.Fatalf("allowed estimate %v want <= %d", estimated, limit)
	}
}

func TestPeek_BufferedSkipStormDoesNotGetEveryCall(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 10
	window := time.Minute
	if _, _, err := limiter.Peek(context.Background(), "k", limit, window); err != nil {
		t.Fatal(err)
	}
	afterSeed := fake.getCallCount()
	if afterSeed == 0 {
		t.Fatal("seed peek sent no GET")
	}
	for i := 0; i < 20; i++ {
		if _, _, peekErr := limiter.Peek(context.Background(), "k", limit, window); peekErr != nil {
			t.Fatal(peekErr)
		}
	}
	if got := fake.getCallCount(); got != afterSeed {
		t.Fatalf("skip storm GET %d want %d (seed only)", got, afterSeed)
	}
}

func TestPeek_ExactGetsEveryCall(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const n = 4
	for i := 0; i < n; i++ {
		if _, _, peekErr := limiter.Peek(context.Background(), "k", 10, time.Minute); peekErr != nil {
			t.Fatal(peekErr)
		}
	}
	if got := fake.getCallCount(); got != 2*n {
		t.Fatalf("exact peek GET %d want %d", got, 2*n)
	}
}

func TestPeek_Unreachable(t *testing.T) {
	client := newSimpleRedisForTest(t, "127.0.0.1:1")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Peek(context.Background(), "k", 1, time.Minute)
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("err %v want %s", err, simpleredis.RedisUnreachable)
	}
}

func wantRedisOutage(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want Redis outage error")
	}
	if isRedisOutageMessage(err.Error()) {
		return
	}
	t.Fatalf("err %v want %s or %s", err, simpleredis.RedisUnreachable, simpleredis.RedisTimeout)
}

// isRedisOutageMessage is Unreachable or Timeout, exact or as a wrapped substring.
func isRedisOutageMessage(msg string) bool {
	return msg == simpleredis.RedisUnreachable || msg == simpleredis.RedisTimeout ||
		strings.Contains(msg, simpleredis.RedisUnreachable) || strings.Contains(msg, simpleredis.RedisTimeout)
}

func TestTake_BufferedPendingDeltaOutage(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take(context.Background(), "k", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	getsAfterSeed := fake.getCallCount()
	if _, _, err := limiter.Take(context.Background(), "k", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	if fake.getCallCount() != getsAfterSeed {
		t.Fatal("buffered Take GETs while the delta is still fresh")
	}
	fake.Kill()
	now = now.Add(time.Hour)
	limiter.SetNowForTest(func() time.Time { return now })
	_, _, err = limiter.Take(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
	_, _, err = limiter.Peek(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
}

func TestTake_BufferedFlushThenKillFailsClosed(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	if _, _, err := limiter.Take(context.Background(), "k", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		limiter.mu.Lock()
		delta := int64(0)
		for _, state := range limiter.windows {
			delta += state.localDelta
		}
		limiter.mu.Unlock()
		if delta == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("flush did not clear localDelta")
		}
		time.Sleep(5 * time.Millisecond)
	}
	fake.Kill()
	_, _, err = limiter.Take(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
}

func TestTake_BufferedTwoInstancesOutage(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	a, err := New(newSimpleRedisForTest(t, addr), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(newSimpleRedisForTest(t, addr), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		a.Close()
		b.Close()
	})
	now := time.Unix(1_700_000_000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	const limit int64 = 5
	window := time.Minute
	nilErrorAdmits := 0
	for _, limiter := range []*Limiter{a, b} {
		allowed, _, takeErr := limiter.Take(context.Background(), "share", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if allowed {
			nilErrorAdmits++
		}
	}
	fake.Kill()
	now = now.Add(time.Hour)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	sawOutage := false
	for _, limiter := range []*Limiter{a, b} {
		for i := 0; i < 4; i++ {
			allowed, _, takeErr := limiter.Take(context.Background(), "share", limit, window)
			if takeErr != nil {
				wantRedisOutage(t, takeErr)
				sawOutage = true
				continue
			}
			if allowed {
				nilErrorAdmits++
			}
		}
	}
	if !sawOutage {
		t.Fatal("want a Redis error after kill")
	}
	if nilErrorAdmits > int(limit) {
		t.Fatalf("nil-error admits %d want <= %d", nilErrorAdmits, limit)
	}
}

func TestParseEvalInt_WrapsCause(t *testing.T) {
	_, err := parseEvalInt([][]byte{[]byte("x")})
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), simpleredis.RedisIssue) {
		t.Fatalf("err %v want %s", err, simpleredis.RedisIssue)
	}
	if !errors.Is(err, strconv.ErrSyntax) {
		t.Fatalf("err %v want wrapped syntax", err)
	}
}

func TestTake_BufferedSleepStoresFlushError(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take(context.Background(), "k", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	fake.Kill()
	limiter.Sleep()
	_, _, err = limiter.Take(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
	_, _, err = limiter.Peek(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
}

func TestPeek_BufferedEmptyFlushThenKill(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take(context.Background(), "k", 5, time.Minute); err != nil {
		t.Fatal(err)
	}
	limiter.Sleep()
	fake.Kill()
	now = now.Add(time.Hour)
	limiter.SetNowForTest(func() time.Time { return now })
	_, _, err = limiter.Peek(context.Background(), "k", 5, time.Minute)
	wantRedisOutage(t, err)
}
