package tokenbucket

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// statusTestNow is the frozen clock every status test reads.
var statusTestNow = time.Unix(1_700_000_000, 0)

// statusStoreForTest is one store under test: its name and its Allow.
type statusStoreForTest struct {
	name  string
	allow func(ctx context.Context, key string) (Status, error)
}

// newStatusStoresForTest builds a memory limiter and a Redis limiter on a fresh fake with the same clock config, both frozen at
// statusTestNow.
func newStatusStoresForTest(t *testing.T, rate float64, burst int64, maxDelay time.Duration) []statusStoreForTest {
	t.Helper()
	mem, err := NewMemory(rate, burst, maxDelay, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetNowForTest(func() time.Time { return statusTestNow })
	_, addr := startTestFakeRedis(t)
	red, err := NewRedis(newSimpleRedisForTest(t, addr), rate, burst, maxDelay, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	red.SetNowForTest(func() time.Time { return statusTestNow })
	return []statusStoreForTest{{name: "memory", allow: mem.Allow}, {name: "redis", allow: red.Allow}}
}

// allowTimesForTest calls Allow count times on the key "k" and returns the last Status.
func allowTimesForTest(t *testing.T, store statusStoreForTest, count int) Status {
	t.Helper()
	var status Status
	for i := 0; i < count; i++ {
		var err error
		status, err = store.allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("%s Allow %d: %v", store.name, i+1, err)
		}
	}
	return status
}

// newEvalReplyLimiterForTest builds a Redis limiter (rate 1, burst 10, maxDelay 1s) whose fake answers every EVAL with wait and
// tokens.
func newEvalReplyLimiterForTest(t *testing.T, wait, tokens string) *Redis {
	t.Helper()
	fake, addr := startTestFakeRedis(t)
	fake.setEvalReply(arrayBulks("true", wait, tokens))
	limiter, err := NewRedis(newSimpleRedisForTest(t, addr), 1, 10, time.Second, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}

func TestStatus_FirstAllowReportsTheBucket(t *testing.T) {
	want := Status{Allowed: true, Limit: 5, Remaining: 4, Period: 500 * time.Millisecond, Reset: 100 * time.Millisecond}
	for _, store := range newStatusStoresForTest(t, 10, 5, 0) {
		if got := allowTimesForTest(t, store, 1); got != want {
			t.Fatalf("%s: got %+v want %+v", store.name, got, want)
		}
	}
}

func TestStatus_FractionalTokensRoundDown(t *testing.T) {
	clock := clockConfig{rate: 1, burst: 5, ttl: testTTL}
	if got := newStatus(clock, 2.7, 0).Remaining; got != 2 {
		t.Fatalf("Remaining %d want 2", got)
	}
}

func TestStatus_AdmittedWithDelayIsDebt(t *testing.T) {
	// Burst 1 at 1/s with maxDelay 2s: the second Allow at the same instant leaves tokens at -1 and waits 1s.
	want := Status{Allowed: true, Limit: 1, Remaining: 0, Period: time.Second, Reset: 2 * time.Second, Delay: time.Second}
	for _, store := range newStatusStoresForTest(t, 1, 1, 2*time.Second) {
		got := allowTimesForTest(t, store, 2)
		if got != want {
			t.Fatalf("%s: got %+v want %+v", store.name, got, want)
		}
		if got.Reset <= got.Period {
			t.Fatalf("%s: debt Reset %v want more than Period %v", store.name, got.Reset, got.Period)
		}
	}
}

func TestStatus_DeniedReportsRetryAfterAndTheRefundedBucket(t *testing.T) {
	// Burst 5 at 1/s with maxDelay 0: the sixth Allow waits 1s, is denied, and its consume is refunded to 0 tokens.
	want := Status{Allowed: false, Limit: 5, Remaining: 0, Period: 5 * time.Second, Reset: 5 * time.Second, RetryAfter: time.Second}
	for _, store := range newStatusStoresForTest(t, 1, 5, 0) {
		if got := allowTimesForTest(t, store, 6); got != want {
			t.Fatalf("%s sixth: got %+v want %+v", store.name, got, want)
		}
		// A following Allow at the same instant sees the refunded bucket, not a stacked consume.
		if got := allowTimesForTest(t, store, 1); got != want {
			t.Fatalf("%s seventh: got %+v want %+v", store.name, got, want)
		}
	}
}

func TestStatus_DeniedAfterDebtReportsTheRefundedDebt(t *testing.T) {
	// Burst 1 at 1/s with maxDelay 2s: Allows 2 and 3 are admitted into debt (-1, -2); Allow 4 waits 3s, is denied, and is
	// refunded back to -2 tokens.
	want := Status{Allowed: false, Limit: 1, Remaining: 0, Period: time.Second, Reset: 3 * time.Second, RetryAfter: 3 * time.Second}
	for _, store := range newStatusStoresForTest(t, 1, 1, 2*time.Second) {
		if got := allowTimesForTest(t, store, 4); got != want {
			t.Fatalf("%s: got %+v want %+v", store.name, got, want)
		}
	}
}

func TestStatus_PeriodSaturatesForAHugeBurstOverATinyRate(t *testing.T) {
	for _, store := range newStatusStoresForTest(t, 0.001, 1e12, 0) {
		got := allowTimesForTest(t, store, 1)
		if got.Period != math.MaxInt64 {
			t.Fatalf("%s: Period %v want the maximum duration", store.name, got.Period)
		}
		if !got.Allowed || got.Period < 0 || got.Reset < 0 || got.Delay < 0 || got.RetryAfter < 0 {
			t.Fatalf("%s: got %+v want allowed and no negative duration", store.name, got)
		}
	}
}

func TestStatus_RetryAfterSaturatesOnAHugeWait(t *testing.T) {
	// At 1e-12 tokens/s the second Allow waits 1e18 microseconds and refilling one token takes as long: more than a duration can hold.
	for _, store := range newStatusStoresForTest(t, 1e-12, 1, 0) {
		got := allowTimesForTest(t, store, 2)
		if got.Allowed || got.RetryAfter != math.MaxInt64 || got.Reset != math.MaxInt64 || got.Delay != 0 {
			t.Fatalf("%s: got %+v want denied with RetryAfter and Reset at the maximum duration", store.name, got)
		}
	}
}

func TestDurationFromNanos_SaturatesAndNeverGoesNegative(t *testing.T) {
	cases := []struct {
		name  string
		nanos float64
		want  time.Duration
	}{
		{name: "zero", nanos: 0, want: 0},
		{name: "negative", nanos: -5, want: 0},
		{name: "nan", nanos: math.NaN(), want: 0},
		{name: "one second", nanos: 1e9, want: time.Second},
		{name: "too large", nanos: 1e21, want: math.MaxInt64},
		{name: "infinity", nanos: math.Inf(1), want: math.MaxInt64},
	}
	for _, tc := range cases {
		if got := durationFromNanos(tc.nanos); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestStatus_ErrorReturnsTheZeroStatus(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, store := range newStatusStoresForTest(t, 1, 1, 0) {
		got, err := store.allow(cancelled, "k")
		if !errors.Is(err, context.Canceled) || got != (Status{}) {
			t.Fatalf("%s cancelled: got %+v err %v", store.name, got, err)
		}
	}
	unreachable, err := NewRedis(newSimpleRedisForTest(t, "127.0.0.1:1"), 1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unreachable.Allow(context.Background(), "k")
	if err == nil || err.Error() != simpleredis.RedisUnreachable || got != (Status{}) {
		t.Fatalf("unreachable: got %+v err %v", got, err)
	}
}

func TestRedis_MalformedTokensIsTokensIsNotANumber(t *testing.T) {
	for _, tokens := range []string{"nan", "+Inf", "-Inf", "inf", "abc"} {
		limiter := newEvalReplyLimiterForTest(t, "0", tokens)
		got, err := limiter.Allow(context.Background(), "k")
		if !errors.Is(err, errEvalTokens) {
			t.Fatalf("tokens %q: err %v want errEvalTokens", tokens, err)
		}
		if got != (Status{}) {
			t.Fatalf("tokens %q: got %+v want the zero Status", tokens, got)
		}
	}
}

func TestRedis_MalformedWaitWinsOverMalformedTokens(t *testing.T) {
	limiter := newEvalReplyLimiterForTest(t, "nan", "abc")
	got, err := limiter.Allow(context.Background(), "k")
	if !errors.Is(err, errEvalWait) || got != (Status{}) {
		t.Fatalf("got %+v err %v want errEvalWait and the zero Status", got, err)
	}
}

func TestRedis_TokensTextFromEitherEngineParses(t *testing.T) {
	// Redis writes 14 significant digits; Dragonfly appends ".0" to whole numbers. Burst is 10, so 1e+15 clamps to Limit.
	cases := []struct {
		tokens string
		want   int64
	}{
		{tokens: "4", want: 4},
		{tokens: "4.0", want: 4},
		{tokens: "-1.0", want: 0},
		{tokens: "0.3", want: 0},
		{tokens: "1e+15", want: 10},
	}
	for _, tc := range cases {
		limiter := newEvalReplyLimiterForTest(t, "0", tc.tokens)
		got, err := limiter.Allow(context.Background(), "k")
		if err != nil {
			t.Fatalf("tokens %q: %v", tc.tokens, err)
		}
		if !got.Allowed || got.Limit != 10 || got.Remaining != tc.want {
			t.Fatalf("tokens %q: got %+v want allowed, Limit 10, Remaining %d", tc.tokens, got, tc.want)
		}
	}
}
