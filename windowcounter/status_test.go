package windowcounter

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// statusModeForTest is one sync_rate mode the status scenarios run in.
type statusModeForTest struct {
	name     string
	syncRate time.Duration
}

// statusModesForTest are exact (sync_rate 0) and buffered (a sync_rate long enough that no flush runs during a test).
var statusModesForTest = []statusModeForTest{
	{name: "exact", syncRate: 0},
	{name: "buffered", syncRate: time.Hour},
}

// statusClockForTest is a settable test clock for one limiter.
type statusClockForTest struct {
	now time.Time
}

// newStatusLimiterForTest builds a limiter in mode on a fresh fake, reading clock.
func newStatusLimiterForTest(t *testing.T, mode statusModeForTest, clock *statusClockForTest) (*Limiter, *testFakeRedis) {
	t.Helper()
	fake, addr := startTestFakeRedis(t)
	limiter, err := New(newSimpleRedisForTest(t, addr), mode.syncRate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(limiter.Close)
	limiter.SetNowForTest(func() time.Time { return clock.now })
	return limiter, fake
}

// windowStartForTest is the start of an epoch-aligned window of the given length near 1_700_000_000.
func windowStartForTest(window time.Duration) time.Time {
	windowSec := int64(window / time.Second)
	return time.Unix(int64(1_700_000_000)/windowSec*windowSec, 0)
}

// takeTimesForTest calls Take count times on key and returns the last Status.
func takeTimesForTest(t *testing.T, limiter *Limiter, key string, limit int64, window time.Duration, count int) Status {
	t.Helper()
	var status Status
	for i := 0; i < count; i++ {
		var err error
		status, err = limiter.Take(context.Background(), key, limit, window)
		if err != nil {
			t.Fatalf("Take %d: %v", i+1, err)
		}
	}
	return status
}

// peekForTest calls Peek once and fails on an error.
func peekForTest(t *testing.T, limiter *Limiter, key string, limit int64, window time.Duration) Status {
	t.Helper()
	status, err := limiter.Peek(context.Background(), key, limit, window)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	return status
}

func TestStatus_TakesInAFreshWindow(t *testing.T) {
	window := time.Minute
	want := Status{Allowed: true, Limit: 10, Remaining: 7, Period: window, Reset: window, Estimated: 3}
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		if got := takeTimesForTest(t, limiter, "k", 10, window, 3); got != want {
			t.Fatalf("%s: got %+v want %+v", mode.name, got, want)
		}
	}
}

func TestStatus_PeekWithAFractionalEstimate(t *testing.T) {
	// One hit in the previous window, seven in this one; 3s into a 10s window the estimate is 7 + 1 x 0.7 = limit - 2.3.
	window := 10 * time.Second
	for _, mode := range statusModesForTest {
		start := windowStartForTest(window)
		clock := &statusClockForTest{now: start.Add(-window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		takeTimesForTest(t, limiter, "k", 10, window, 1)
		clock.now = start.Add(3 * time.Second)
		takeTimesForTest(t, limiter, "k", 10, window, 7)
		got := peekForTest(t, limiter, "k", 10, window)
		if !got.Allowed || got.Remaining != 2 || got.Estimated < 7.69 || got.Estimated > 7.71 {
			t.Fatalf("%s: got %+v want allowed, Remaining 2, Estimated 7.7", mode.name, got)
		}
	}
}

func TestStatus_OverTheLimitHasNothingRemaining(t *testing.T) {
	window := time.Minute
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		got := takeTimesForTest(t, limiter, "k", 2, window, 4)
		if got.Allowed || got.Estimated != 4 || got.Remaining != 0 {
			t.Fatalf("%s: got %+v want denied, Estimated 4, Remaining 0", mode.name, got)
		}
	}
}

func TestStatus_EmptyWindowHasTheWholeLimitRemaining(t *testing.T) {
	// With no hits Remaining is the whole limit, also when float64(limit) rounds above the largest int64.
	window := time.Minute
	for _, mode := range statusModesForTest {
		for _, limit := range []int64{10, math.MaxInt64} {
			clock := &statusClockForTest{now: windowStartForTest(window)}
			limiter, _ := newStatusLimiterForTest(t, mode, clock)
			got := peekForTest(t, limiter, "k", limit, window)
			if !got.Allowed || got.Limit != limit || got.Remaining != limit {
				t.Fatalf("%s limit %d: got %+v want allowed with Remaining == Limit", mode.name, limit, got)
			}
		}
	}
}

func TestStatus_ResetIsTheFixedWindowEnd(t *testing.T) {
	window := time.Minute
	for _, mode := range statusModesForTest {
		start := windowStartForTest(window)
		clock := &statusClockForTest{now: start}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		// At the window start Reset is the whole window.
		if got := takeTimesForTest(t, limiter, "k", 10, window, 1).Reset; got != window {
			t.Fatalf("%s: Reset at the window start %v want %v", mode.name, got, window)
		}
		// 15.25s in, Reset keeps the fraction of a second for Take and for Peek.
		clock.now = start.Add(15*time.Second + 250*time.Millisecond)
		if got := takeTimesForTest(t, limiter, "k", 10, window, 1).Reset; got != 44750*time.Millisecond {
			t.Fatalf("%s: Take Reset %v want 44.75s", mode.name, got)
		}
		if got := peekForTest(t, limiter, "k", 10, window).Reset; got != 44750*time.Millisecond {
			t.Fatalf("%s: Peek Reset %v want 44.75s", mode.name, got)
		}
	}
}

func TestStatus_DeniedTakeRetriesInTheSameWindow(t *testing.T) {
	window := 10 * time.Second
	for _, mode := range statusModesForTest {
		start := windowStartForTest(window)
		clock := &statusClockForTest{now: start.Add(-window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		// The previous window holds 10 hits; at the start of this window one more Take is 1 + 10 = 11.
		takeTimesForTest(t, limiter, "k", 10, window, 10)
		clock.now = start
		denied := takeTimesForTest(t, limiter, "k", 10, window, 1)
		if denied.Allowed || denied.Estimated != 11 || denied.Remaining != 0 || denied.RetryAfter != 2*time.Second {
			t.Fatalf("%s: got %+v want denied, Estimated 11, Remaining 0, RetryAfter 2s", mode.name, denied)
		}
		// 2s later the weight is 0.8: 2 + 10 x 0.8 = 10.
		clock.now = start.Add(denied.RetryAfter)
		if retried := takeTimesForTest(t, limiter, "k", 10, window, 1); !retried.Allowed {
			t.Fatalf("%s: Take at RetryAfter denied: %+v", mode.name, retried)
		}
	}
}

func TestStatus_DeniedTakeRetriesInTheNextWindow(t *testing.T) {
	window := 10 * time.Second
	for _, mode := range statusModesForTest {
		start := windowStartForTest(window)
		clock := &statusClockForTest{now: start.Add(5 * time.Second)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		denied := takeTimesForTest(t, limiter, "k", 2, window, 3)
		if denied.Allowed || denied.RetryAfter != 12*time.Second {
			t.Fatalf("%s: got %+v want denied with RetryAfter 12s", mode.name, denied)
		}
		// 7s into the next window this window's 3 hits weigh 0.3: 1 + 0.9 is at most 2.
		clock.now = clock.now.Add(denied.RetryAfter)
		if retried := takeTimesForTest(t, limiter, "k", 2, window, 1); !retried.Allowed {
			t.Fatalf("%s: Take at RetryAfter denied: %+v", mode.name, retried)
		}
	}
}

func TestStatus_DeniedPeekRetriesLikeTheNextTake(t *testing.T) {
	window := 10 * time.Second
	for _, mode := range statusModesForTest {
		start := windowStartForTest(window)
		clock := &statusClockForTest{now: start.Add(5 * time.Second)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		takeTimesForTest(t, limiter, "k", 2, window, 3)
		got := peekForTest(t, limiter, "k", 2, window)
		if got.Allowed || got.Remaining != 0 || got.RetryAfter != 12*time.Second {
			t.Fatalf("%s: got %+v want denied, Remaining 0, RetryAfter 12s", mode.name, got)
		}
	}
}

func TestStatus_PeekAtExactlyTheLimit(t *testing.T) {
	window := time.Minute
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		takeTimesForTest(t, limiter, "k", 2, window, 2)
		peeked := peekForTest(t, limiter, "k", 2, window)
		if !peeked.Allowed || peeked.Remaining != 0 || peeked.RetryAfter != 0 {
			t.Fatalf("%s: Peek got %+v want allowed, Remaining 0, RetryAfter 0", mode.name, peeked)
		}
		if taken := takeTimesForTest(t, limiter, "k", 2, window, 1); taken.Allowed {
			t.Fatalf("%s: Take after Peek at the limit allowed: %+v", mode.name, taken)
		}
	}
}

func TestStatus_NegativeLimitHasNothingRemaining(t *testing.T) {
	window := time.Minute
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(window)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		got := peekForTest(t, limiter, "k", -1, window)
		if got.Allowed || got.Remaining != 0 || got.RetryAfter != got.Reset {
			t.Fatalf("%s: got %+v want denied, Remaining 0, RetryAfter == Reset", mode.name, got)
		}
	}
}

func TestStatus_LimitBelowOneRetriesAtTheWindowEnd(t *testing.T) {
	window := time.Minute
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(window).Add(20 * time.Second)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		taken := takeTimesForTest(t, limiter, "k", 0, window, 1)
		if taken.Allowed || taken.RetryAfter != taken.Reset || taken.RetryAfter != 40*time.Second {
			t.Fatalf("%s: Take got %+v want denied with RetryAfter == Reset == 40s", mode.name, taken)
		}
		peeked := peekForTest(t, limiter, "k", 0, window)
		if peeked.Allowed || peeked.RetryAfter != peeked.Reset {
			t.Fatalf("%s: Peek got %+v want denied with RetryAfter == Reset", mode.name, peeked)
		}
	}
}

func TestStatus_BufferedOutageReportsTheLocalCap(t *testing.T) {
	window := time.Minute
	clock := &statusClockForTest{now: windowStartForTest(window)}
	limiter, fake := newStatusLimiterForTest(t, statusModesForTest[1], clock)
	fake.Kill()
	const limit int64 = 3
	takeTimesForTest(t, limiter, "k", limit, window, int(limit-1))
	got, err := limiter.Take(context.Background(), "k", limit, window)
	if err != nil {
		t.Fatalf("Take during outage err %v want nil", err)
	}
	if !got.Allowed || got.Remaining != 0 || got.Estimated != float64(limit) {
		t.Fatalf("Take during outage got %+v want allowed, Remaining 0, Estimated %d", got, limit)
	}
}

func TestStatus_ExactModeSendsOnlyItsCommands(t *testing.T) {
	window := time.Minute
	clock := &statusClockForTest{now: windowStartForTest(window)}
	limiter, fake := newStatusLimiterForTest(t, statusModesForTest[0], clock)
	fake.enableScriptCache()
	takeTimesForTest(t, limiter, "warm", 1, window, 1)
	// A denied Take computes RetryAfter and still sends one EVALSHA and one GET; a denied Peek sends two GETs.
	before := fake.commandCount()
	if got := takeTimesForTest(t, limiter, "warm", 1, window, 1); got.Allowed || got.RetryAfter <= 0 {
		t.Fatalf("Take got %+v want denied with a RetryAfter", got)
	}
	if sent := fake.commandCount() - before; sent != 2 {
		t.Fatalf("denied Take sent %d commands want 2", sent)
	}
	before = fake.commandCount()
	if got := peekForTest(t, limiter, "warm", 1, window); got.Allowed || got.RetryAfter <= 0 {
		t.Fatalf("Peek got %+v want denied with a RetryAfter", got)
	}
	if sent := fake.commandCount() - before; sent != 2 {
		t.Fatalf("denied Peek sent %d commands want 2", sent)
	}
}

func TestStatus_ErrorReturnsTheZeroStatus(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, mode := range statusModesForTest {
		clock := &statusClockForTest{now: windowStartForTest(time.Minute)}
		limiter, _ := newStatusLimiterForTest(t, mode, clock)
		if got, err := limiter.Take(cancelled, "k", 1, time.Minute); !errors.Is(err, context.Canceled) || got != (Status{}) {
			t.Fatalf("%s cancelled Take: got %+v err %v", mode.name, got, err)
		}
		if got, err := limiter.Peek(cancelled, "k", 1, time.Minute); !errors.Is(err, context.Canceled) || got != (Status{}) {
			t.Fatalf("%s cancelled Peek: got %+v err %v", mode.name, got, err)
		}
		if got, err := limiter.Take(context.Background(), "k", 1, 1500*time.Millisecond); err == nil || got != (Status{}) {
			t.Fatalf("%s fractional window: got %+v err %v", mode.name, got, err)
		}
	}
	unreachable, err := New(newSimpleRedisForTest(t, "127.0.0.1:1"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := unreachable.Take(context.Background(), "k", 1, time.Minute); err == nil || got != (Status{}) {
		t.Fatalf("exact unreachable Take: got %+v err %v", got, err)
	}
}

// admitsAtSecondForTest is whether one more Take at Unix second retrySecond is admitted, with no traffic after a call that saw
// current and previous counts in the window starting at windowStart.
func admitsAtSecondForTest(current, previous, limit, windowStart, windowSec, retrySecond int64) bool {
	windowEnd := windowStart + windowSec
	switch {
	case retrySecond < windowEnd:
		return admitsNextTake(current, previous, limit, retrySecond-windowStart, windowSec)
	case retrySecond < windowEnd+windowSec:
		return admitsNextTake(0, current, limit, retrySecond-windowEnd, windowSec)
	default:
		return admitsNextTake(0, 0, limit, 0, windowSec)
	}
}

func TestStatus_RetrySecondIsTheFirstAdmittingSecond(t *testing.T) {
	// Every denied combination on a grid: the reported second admits, the second before it does not, and it lies after the
	// call's own second and within two windows.
	for _, windowSec := range []int64{1, 2, 3, 7, 10, 60} {
		windowStart := int64(1_700_000_000) / windowSec * windowSec
		for limit := int64(1); limit <= 12; limit++ {
			for current := int64(0); current <= 14; current++ {
				for previous := int64(0); previous <= 14; previous++ {
					for elapsed := int64(0); elapsed < windowSec; elapsed++ {
						now := time.Unix(windowStart+elapsed, 0)
						sliding := slidingWindow{weight: weightAt(elapsed, windowSec), now: now, windowSec: windowSec, windowStart: windowStart}
						status := newStatus(sliding, limit, current, previous)
						if status.Allowed {
							continue
						}
						retrySecond := now.Add(status.RetryAfter).Unix()
						if status.RetryAfter <= 0 || status.RetryAfter > 2*time.Duration(windowSec)*time.Second {
							t.Fatalf("W %d L %d C %d P %d e %d: RetryAfter %v out of (0, 2W]", windowSec, limit, current, previous, elapsed, status.RetryAfter)
						}
						if !admitsAtSecondForTest(current, previous, limit, windowStart, windowSec, retrySecond) {
							t.Fatalf("W %d L %d C %d P %d e %d: retry second %d does not admit", windowSec, limit, current, previous, elapsed, retrySecond-windowStart)
						}
						if admitsAtSecondForTest(current, previous, limit, windowStart, windowSec, retrySecond-1) {
							t.Fatalf("W %d L %d C %d P %d e %d: second %d before the retry already admits", windowSec, limit, current, previous, elapsed, retrySecond-1-windowStart)
						}
					}
				}
			}
		}
	}
}
