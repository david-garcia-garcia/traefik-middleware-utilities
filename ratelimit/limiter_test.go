package ratelimit

import (
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

func TestTake_NThenDeny(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	const limit int64 = 3
	window := time.Minute
	for i := int64(0); i < limit; i++ {
		allowed, estimated, takeErr := limiter.Take("k", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("take %d denied, estimated %v", i+1, estimated)
		}
	}
	allowed, estimated, err := limiter.Take("k", limit, window)
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
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Second
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take("k", 5, window); err != nil {
		t.Fatal(err)
	}
	got := fake.lastExpireCommand()
	if len(got) < 3 || got[0] != "EXPIRE" || got[2] != "20" {
		t.Fatalf("expire argv %v want EXPIRE … 20", got)
	}
}

func TestTake_SlidingBoundaryDoesNotDouble(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
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
		allowed, _, takeErr := limiter.Take("k", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("fill take %d denied", i+1)
		}
	}
	now = start.Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, estimated, err := limiter.Take("k", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("boundary take allowed, estimated %v (fixed-window double)", estimated)
	}
}

func TestTake_Unreachable(t *testing.T) {
	client := &simpleredis.SimpleRedis{}
	client.Init("127.0.0.1:1", "", "")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = limiter.Take("k", 1, time.Minute)
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("err %v want %s", err, simpleredis.RedisUnreachable)
	}
}

func TestNew_NegativeSyncRate(t *testing.T) {
	client := &simpleredis.SimpleRedis{}
	if _, err := New(client, -time.Second); err == nil {
		t.Fatal("want error")
	}
}

func TestBuffered_TwoClientsShareWithoutLastWriteWins(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := &simpleredis.SimpleRedis{}
	aClient.Init(addr, "", "")
	bClient := &simpleredis.SimpleRedis{}
	bClient.Init(addr, "", "")
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
		allowed, _, takeErr := a.Take("share", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("a take %d denied", i+1)
		}
		allowed, _, takeErr = b.Take("share", limit, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("b take %d denied", i+1)
		}
	}
	a.Sleep()
	b.Sleep()
	allowed, estimated, err := a.Take("share", limit, window)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("shared count last-write-wins? allowed estimated %v", estimated)
	}
}

func TestClose_StopsTickerAndKeepsRedis(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	limiter.Sleep()
	limiter.Close()
	if _, err := client.Incr("still-open"); err != nil {
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

func TestAllow_IsTake(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}
	allowed, _, err := limiter.Allow("k", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first Allow denied")
	}
}
