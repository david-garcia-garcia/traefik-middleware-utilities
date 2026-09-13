package windowcounter

import (
	"context"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestRepro_BufferedTakeHoldsLockDuringRedisGet fails if a buffered Take on key "fast"
// waits for an in-flight Redis GET on a different opaque key. That is HOLB: takeBuffered
// holds l.mu across getCount.
func TestRepro_BufferedTakeHoldsLockDuringRedisGet(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	const getDelay = 400 * time.Millisecond
	fake.blockGetPrefix("slow:", getDelay)

	client, err := simpleredis.New(simpleredis.Config{
		Host:        addr,
		MaxRetries:  -1,
		IOTimeout:   5 * time.Second,
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	limiter, err := New(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fake.unblockGet()
		limiter.Close()
	})

	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	go func() {
		_, _, _ = limiter.Take(context.Background(), "slow", 100, time.Minute)
	}()
	if !fake.waitGetBlocked(5 * time.Second) {
		t.Fatal("GET on slow never started")
	}

	started := time.Now()
	done := make(chan error, 1)
	go func() {
		_, _, takeErr := limiter.Take(context.Background(), "fast", 100, time.Minute)
		done <- takeErr
	}()

	select {
	case takeErr := <-done:
		wait := time.Since(started)
		if takeErr != nil {
			t.Fatalf("fast Take err %v after %v (GET delay %v)", takeErr, wait, getDelay)
		}
		if wait > 200*time.Millisecond {
			t.Fatalf("fast Take waited %v while GET on unrelated key was delayed %v (limiter mutex held across Redis GET)", wait, getDelay)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("fast Take still blocked after 5s (GET delay %v); limiter mutex held across Redis GET", getDelay)
	}
}
