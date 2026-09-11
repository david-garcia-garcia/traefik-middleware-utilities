package tokenbucket

import (
	"errors"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const testTTL = 2 * time.Second
const agreeMaxDelay = time.Millisecond

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
	if _, _, err := limiter.Allow("k"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(testTTL)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 2; i++ {
		allowed, wait, allowErr := limiter.Allow("k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed || wait != 0 {
			t.Fatalf("fresh burst %d: allowed %v wait %v", i+1, allowed, wait)
		}
	}
}

func TestRedis_EvalBadReply(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	fake.setEvalReply("*1\r\n$4\r\ntrue\r\n")
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 1, time.Second, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Allow("k")
	if !errors.Is(err, errEvalLen) {
		t.Fatalf("want errEvalLen, got %v", err)
	}
	fake.setEvalReply("*3\r\n$4\r\ntrue\r\n$3\r\nxyz\r\n$1\r\n0\r\n")
	_, _, err = limiter.Allow("k")
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
		allowed, wait, allowErr := limiter.Allow("k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed || wait != 0 {
			t.Fatalf("burst %d: allowed %v wait %v", i+1, allowed, wait)
		}
	}
	allowed, wait, err := limiter.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("want delayed admit when maxDelay is large")
	}
	if wait <= 0 {
		t.Fatalf("wait %v want > 0", wait)
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
		allowed, _, allowErr := limiter.Allow("k")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed {
			t.Fatalf("burst %d denied", i+1)
		}
	}
	allowed, wait, err := limiter.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed || wait <= time.Microsecond {
		t.Fatalf("want deny wait>maxDelay, got allowed %v wait %v", allowed, wait)
	}
	allowedAfterRefund, waitAfterRefund, err := limiter.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if allowedAfterRefund {
		t.Fatal("refund must not admit a stacked consume")
	}
	if waitAfterRefund != wait {
		t.Fatalf("wait after refund %v want %v (not stacked)", waitAfterRefund, wait)
	}
}

func TestRedis_EvalEncoding(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 3, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Allow("src"); err != nil {
		t.Fatal(err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 9 || got[0] != "EVAL" || got[2] != "1" || got[3] != "src" {
		t.Fatalf("eval argv %v", got)
	}
}

func TestRedis_TwoInstancesShareBurst(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := &simpleredis.SimpleRedis{}
	aClient.Init(addr, "", "")
	bClient := &simpleredis.SimpleRedis{}
	bClient.Init(addr, "", "")
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
		allowed, _, allowErr := a.Allow("shared")
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !allowed {
			t.Fatalf("a burst %d denied", i+1)
		}
	}
	allowed, _, err := b.Allow("shared")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("second instance granted another burst")
	}
}

func TestMemoryAndRedis_Agree(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
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
		mAllowed, mWait, mErr := mem.Allow("k")
		if mErr != nil {
			t.Fatal(mErr)
		}
		rAllowed, rWait, rErr := red.Allow("k")
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
}

func TestRedis_Unreachable(t *testing.T) {
	client := &simpleredis.SimpleRedis{}
	client.Init("127.0.0.1:1", "", "")
	limiter, err := NewRedis(client, 1, 1, time.Second, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Allow("k")
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("want redis:unreachable, got %v", err)
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
