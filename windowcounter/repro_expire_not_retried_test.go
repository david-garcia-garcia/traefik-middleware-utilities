package windowcounter

import (
	"context"
	"testing"
	"time"
)

// TestRepro_ExactExpireNotRetriedAfterFailure proves takeExact never retries EXPIRE after the first-hit EXPIRE fails.
func TestRepro_ExactExpireNotRetriedAfterFailure(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	fake.failNextExpireCommands(1)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Second
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	_, _, firstErr := limiter.Take(context.Background(), "k", 5, window)
	if firstErr == nil {
		t.Fatal("first Take: Expire failed, want error")
	}
	afterFirst := fake.expireCommandCount()
	if afterFirst != 1 {
		t.Fatalf("expire commands after first Take = %d, want 1", afterFirst)
	}

	_, _, secondErr := limiter.Take(context.Background(), "k", 5, window)
	afterSecond := fake.expireCommandCount()
	if afterSecond == afterFirst {
		t.Fatalf("second Take did not send EXPIRE (count still %d after Incr=2); second Take err=%v", afterSecond, secondErr)
	}
	if secondErr != nil {
		t.Fatalf("second Take err %v, want nil after EXPIRE retry succeeded", secondErr)
	}
}
