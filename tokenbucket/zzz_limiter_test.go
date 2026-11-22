package tokenbucket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const testTTL = 2 * time.Second
const agreeMaxDelay = time.Millisecond

// newSimpleRedisForTest returns a New client for tests.
func newSimpleRedisForTest(tb testing.TB, host string) *simpleredis.SimpleRedis {
	tb.Helper()
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		tb.Fatal(err)
	}
	return client
}

func TestNewRedis_RejectsNil(t *testing.T) {
	limiter, err := NewRedis(nil, 1, 1, time.Second, testTTL)
	if limiter != nil || !errors.Is(err, errRedis) {
		t.Fatalf("nil redis: limiter %v err %v", limiter, err)
	}
}

func TestMemory_IdlePastTTLStartsFull(t *testing.T) {
	limiter, err := NewMemory(1, 2, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, err := limiter.Allow(context.Background(), "k"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(testTTL)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 2; i++ {
		status, allowErr := limiter.Allow(context.Background(), "k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !status.Allowed || (status.Delay+status.RetryAfter) != 0 {
			t.Fatalf("fresh burst %d: allowed %v wait %v", i+1, status.Allowed, status.Delay+status.RetryAfter)
		}
	}
}

func TestRedis_EvalBadReply(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	fake.setEvalReply("*1\r\n$4\r\ntrue\r\n")
	client := newSimpleRedisForTest(t, addr)
	limiter, err := NewRedis(client, 1, 1, time.Second, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = limiter.Allow(context.Background(), "k")
	if !errors.Is(err, errEvalLen) {
		t.Fatalf("want errEvalLen, got %v", err)
	}
	fake.setEvalReply("*3\r\n$4\r\ntrue\r\n$3\r\nxyz\r\n$1\r\n0\r\n")
	_, err = limiter.Allow(context.Background(), "k")
	if !errors.Is(err, errEvalWait) {
		t.Fatalf("want errEvalWait, got %v", err)
	}
}

func TestNewMemory_RejectsInvalidClock(t *testing.T) {
	if _, err := NewMemory(0, 1, 0, testTTL); !errors.Is(err, errRate) {
		t.Fatalf("rate 0: %v", err)
	}
	if _, err := NewMemory(1, 0, 0, testTTL); !errors.Is(err, errBurst) {
		t.Fatalf("burst 0: %v", err)
	}
	if _, err := NewMemory(1, 1, -time.Second, testTTL); !errors.Is(err, errDelay) {
		t.Fatalf("negative delay: %v", err)
	}
	if _, err := NewMemory(1, 1, 0, time.Millisecond); !errors.Is(err, errTTL) {
		t.Fatalf("ttl < 1s: %v", err)
	}
}

func TestMemory_BurstAfterIdle(t *testing.T) {
	limiter, err := NewMemory(1, 3, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := limiter.Allow(context.Background(), "k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !status.Allowed || (status.Delay+status.RetryAfter) != 0 {
			t.Fatalf("burst %d: allowed %v wait %v", i+1, status.Allowed, status.Delay+status.RetryAfter)
		}
	}
	status, err := limiter.Allow(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Allowed {
		t.Fatal("want delayed admit when maxDelay is large")
	}
	if (status.Delay + status.RetryAfter) <= 0 {
		t.Fatalf("wait %v want > 0", status.Delay+status.RetryAfter)
	}
}

func TestMemory_RefundWhenWaitExceedsMaxDelay(t *testing.T) {
	limiter, err := NewMemory(1, 3, time.Microsecond, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := limiter.Allow(context.Background(), "k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !status.Allowed {
			t.Fatalf("burst %d denied", i+1)
		}
	}
	status, err := limiter.Allow(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if status.Allowed || (status.Delay+status.RetryAfter) <= time.Microsecond {
		t.Fatalf("want deny wait>maxDelay, got allowed %v wait %v", status.Allowed, status.Delay+status.RetryAfter)
	}
	statusAfterRefund, err := limiter.Allow(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if statusAfterRefund.Allowed {
		t.Fatal("refund must not admit a stacked consume")
	}
	if (statusAfterRefund.Delay + statusAfterRefund.RetryAfter) != (status.Delay + status.RetryAfter) {
		t.Fatalf("wait after refund %v want %v (not stacked)", statusAfterRefund.Delay+statusAfterRefund.RetryAfter, status.Delay+status.RetryAfter)
	}
}

func TestRedis_EvalEncoding(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := NewRedis(client, 1, 3, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, err := limiter.Allow(context.Background(), "src"); err != nil {
		t.Fatal(err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 9 || got[0] != "EVAL" || got[2] != "1" || got[3] != "src" {
		t.Fatalf("eval argv %v", got)
	}
}

func TestRedis_TwoInstancesShareBurst(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := newSimpleRedisForTest(t, addr)
	bClient := newSimpleRedisForTest(t, addr)
	a, err := NewRedis(aClient, 1, 3, time.Microsecond, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRedis(bClient, 1, 3, time.Microsecond, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := a.Allow(context.Background(), "shared")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !status.Allowed {
			t.Fatalf("a burst %d denied", i+1)
		}
	}
	status, err := b.Allow(context.Background(), "shared")
	if err != nil {
		t.Fatal(err)
	}
	if status.Allowed {
		t.Fatal("second instance granted another burst")
	}
}

func TestMemoryAndRedis_Agree(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	mem, err := NewMemory(2, 2, agreeMaxDelay, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	red, err := NewRedis(client, 2, 2, agreeMaxDelay, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	mem.SetNowForTest(func() time.Time { return now })
	red.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 4; i++ {
		mStatus, mErr := mem.Allow(context.Background(), "k")
		if mErr != nil {
			t.Fatal(mErr)
		}
		rStatus, rErr := red.Allow(context.Background(), "k")
		if rErr != nil {
			t.Fatal(rErr)
		}
		assertStatusAgrees(t, i, mStatus, rStatus)
	}
}

func TestRedis_Unreachable(t *testing.T) {
	client := newSimpleRedisForTest(t, "127.0.0.1:1")
	limiter, err := NewRedis(client, 1, 1, time.Second, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = limiter.Allow(context.Background(), "k")
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("want redis:unreachable, got %v", err)
	}
}

// assertStatusAgrees fails when one Allow step's memory and Redis statuses differ: Allowed and the bucket fields exactly (the
// sequences keep tokens whole), Delay and RetryAfter by waitClass.
func assertStatusAgrees(t *testing.T, step int, mStatus, rStatus Status) {
	t.Helper()
	if mStatus.Allowed != rStatus.Allowed {
		t.Fatalf("step %d allowed mem %v redis %v", step, mStatus.Allowed, rStatus.Allowed)
	}
	if mStatus.Limit != rStatus.Limit || mStatus.Remaining != rStatus.Remaining || mStatus.Period != rStatus.Period || mStatus.Reset != rStatus.Reset {
		t.Fatalf("step %d bucket mem %+v redis %+v", step, mStatus, rStatus)
	}
	if waitClass(mStatus.Delay) != waitClass(rStatus.Delay) || waitClass(mStatus.RetryAfter) != waitClass(rStatus.RetryAfter) {
		t.Fatalf("step %d wait mem %+v redis %+v", step, mStatus, rStatus)
	}
}

// waitClass is zero, positive-at-most-agreeMaxDelay, or greater-than-agreeMaxDelay.
func waitClass(wait time.Duration) int {
	if wait == 0 {
		return 0
	}
	if wait <= agreeMaxDelay {
		return 1
	}
	return 2
}
