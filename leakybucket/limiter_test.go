package leakybucket

import (
	"errors"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const testTTL = 2 * time.Second

func TestNewMemory_RejectsInvalidClock(t *testing.T) {
	if _, err := NewMemory(0, 1, testTTL); !errors.Is(err, errLeak) {
		t.Fatalf("leak 0: %v", err)
	}
	if _, err := NewMemory(-1, 1, testTTL); !errors.Is(err, errLeak) {
		t.Fatalf("negative leak: %v", err)
	}
	if _, err := NewMemory(1, 0, testTTL); !errors.Is(err, errCapacity) {
		t.Fatalf("capacity 0: %v", err)
	}
	if _, err := NewMemory(1, -1, testTTL); !errors.Is(err, errCapacity) {
		t.Fatalf("negative capacity: %v", err)
	}
	if _, err := NewMemory(1, 1, time.Millisecond); !errors.Is(err, errTTL) {
		t.Fatalf("ttl < 1s: %v", err)
	}
}

func TestMemory_AddRejectsNLessThanOne(t *testing.T) {
	limiter, err := NewMemory(1, 3, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, addErr := limiter.Add("k", 0); !errors.Is(addErr, errPour) {
		t.Fatalf("n 0: %v", addErr)
	}
	if _, _, _, addErr := limiter.Add("k", -1); !errors.Is(addErr, errPour) {
		t.Fatalf("n -1: %v", addErr)
	}
	level, err := limiter.Level("k")
	if err != nil {
		t.Fatal(err)
	}
	if level != 0 {
		t.Fatalf("rejected pour stored water %v", level)
	}
}

func TestMemory_PourToCapacityThenDeny(t *testing.T) {
	limiter, err := NewMemory(1, 3, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, level, _, err := limiter.Add("opaque", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || level != 3 {
		t.Fatalf("pour to cap: allowed %v level %v", allowed, level)
	}
	allowed, level, _, err = limiter.Take("opaque")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("take after cap allowed")
	}
	if level != 3 {
		t.Fatalf("denied water %v want 3", level)
	}
}

func TestMemory_IdleDrainsWater(t *testing.T) {
	limiter, err := NewMemory(1, 3, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, _, err := limiter.Add("k", 3); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	level, err := limiter.Level("k")
	if err != nil {
		t.Fatal(err)
	}
	if waterClass(level, 3) != 0 {
		t.Fatalf("idle level %v want empty", level)
	}
	allowed, _, _, err := limiter.Take("k")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("take after drain denied")
	}
}

func TestMemory_LevelAfterDeltaT(t *testing.T) {
	limiter, err := NewMemory(1, 3, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, _, err := limiter.Add("k", 3); err != nil {
		t.Fatal(err)
	}
	now = now.Add(1500 * time.Millisecond)
	limiter.SetNowForTest(func() time.Time { return now })
	level, err := limiter.Level("k")
	if err != nil {
		t.Fatal(err)
	}
	if level < 1.4 || level > 1.6 {
		t.Fatalf("level after half drain %v want ~1.5", level)
	}
	allowed, afterTake, _, err := limiter.Take("k")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("take after level denied (level poured?)")
	}
	if afterTake < 2.4 || afterTake > 2.6 {
		t.Fatalf("take after level water %v want ~2.5", afterTake)
	}
}

func TestMemory_UntilNotFull(t *testing.T) {
	limiter, err := NewMemory(1, 3, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, until, err := limiter.Add("k", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || until != 0 {
		t.Fatalf("room for one more: allowed %v until %v", allowed, until)
	}
	allowed, _, until, err = limiter.Add("k", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("fill denied")
	}
	if until != time.Second {
		t.Fatalf("until-not-full %v want 1s", until)
	}
}

func TestMemory_IdlePastTTLStartsEmpty(t *testing.T) {
	limiter, err := NewMemory(1, 2, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, _, err := limiter.Add("k", 2); err != nil {
		t.Fatal(err)
	}
	now = now.Add(testTTL)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, level, _, err := limiter.Add("k", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || level != 2 {
		t.Fatalf("fresh after ttl: allowed %v level %v", allowed, level)
	}
}

func TestNewRedis_RejectsNil(t *testing.T) {
	limiter, err := NewRedis(nil, 1, 1, 0, testTTL)
	if limiter != nil || !errors.Is(err, errRedis) {
		t.Fatalf("nil redis: limiter %v err %v", limiter, err)
	}
}

func TestNewRedis_RejectsNegativeSyncRate(t *testing.T) {
	client := &simpleredis.SimpleRedis{}
	if _, err := NewRedis(client, 1, 1, -time.Second, testTTL); !errors.Is(err, errSyncRate) {
		t.Fatalf("negative sync: %v", err)
	}
}

func TestNewRedis_FloorsSyncRateBelow20ms(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 1, time.Millisecond, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	if limiter.syncRate != minSyncRate {
		t.Fatalf("syncRate %v want %v", limiter.syncRate, minSyncRate)
	}
}

func TestRedis_EvalBadReply(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	fake.setEvalReply("*1\r\n$1\r\n1\r\n")
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = limiter.Take("k")
	if !errors.Is(err, errEvalLen) {
		t.Fatalf("want errEvalLen, got %v", err)
	}
	fake.setEvalReply("*3\r\n$1\r\n1\r\n$3\r\nxyz\r\n$1\r\n0\r\n")
	_, _, _, err = limiter.Take("k")
	if !errors.Is(err, errEvalWater) {
		t.Fatalf("want errEvalWater, got %v", err)
	}
	fake.setEvalReply("*3\r\n$1\r\n2\r\n$1\r\n0\r\n$1\r\n0\r\n")
	_, _, _, err = limiter.Take("k")
	if !errors.Is(err, errEvalFlag) {
		t.Fatalf("want errEvalFlag, got %v", err)
	}
}

func TestRedis_EvalEncoding(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 3, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, _, err := limiter.Take("src"); err != nil {
		t.Fatal(err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 9 || got[0] != "EVAL" || got[2] != "1" || got[3] != "src" {
		t.Fatalf("eval argv %v", got)
	}
}

func TestRedis_TwoInstancesBothPoursCount(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := &simpleredis.SimpleRedis{}
	aClient.Init(addr, "", "")
	bClient := &simpleredis.SimpleRedis{}
	bClient.Init(addr, "", "")
	a, err := NewRedis(aClient, 1, 3, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRedis(bClient, 1, 3, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(); b.Close() })
	now := time.Unix(1_700_000_000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	if allowed, _, _, takeErr := a.Take("shared"); takeErr != nil || !allowed {
		t.Fatalf("a take: allowed %v err %v", allowed, takeErr)
	}
	if allowed, _, _, takeErr := b.Take("shared"); takeErr != nil || !allowed {
		t.Fatalf("b take: allowed %v err %v", allowed, takeErr)
	}
	a.Sleep()
	b.Sleep()
	reader, err := NewRedis(aClient, 1, 3, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	reader.SetNowForTest(func() time.Time { return now })
	level, err := reader.Level("shared")
	if err != nil {
		t.Fatal(err)
	}
	if level != 2 {
		t.Fatalf("shared water %v want 2 (last-write-wins?)", level)
	}
}

func TestRedis_OverAllowBoundedBySyncInterval(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	aClient := &simpleredis.SimpleRedis{}
	aClient.Init(addr, "", "")
	bClient := &simpleredis.SimpleRedis{}
	bClient.Init(addr, "", "")
	a, err := NewRedis(aClient, 1, 2, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRedis(bClient, 1, 2, time.Hour, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(); b.Close() })
	now := time.Unix(1_700_000_000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 2; i++ {
		if allowed, _, _, takeErr := a.Take("k"); takeErr != nil || !allowed {
			t.Fatalf("a fill %d: allowed %v err %v", i+1, allowed, takeErr)
		}
		if allowed, _, _, takeErr := b.Take("k"); takeErr != nil || !allowed {
			t.Fatalf("b fill %d: allowed %v err %v", i+1, allowed, takeErr)
		}
	}
	now = now.Add(time.Hour)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	if allowed, _, _, takeErr := a.Take("k"); takeErr != nil {
		t.Fatal(takeErr)
	} else if allowed {
		t.Fatal("a over-allow grew without a flush")
	}
	if allowed, _, _, takeErr := b.Take("k"); takeErr != nil {
		t.Fatal(takeErr)
	} else if allowed {
		t.Fatal("b over-allow grew without a flush")
	}
	a.Sleep()
	b.Sleep()
	reader, err := NewRedis(aClient, 1, 2, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	reader.SetNowForTest(func() time.Time { return now })
	if allowed, _, _, takeErr := reader.Take("k"); takeErr != nil {
		t.Fatal(takeErr)
	} else if allowed {
		t.Fatal("shared water after flush still admitted")
	}
}

func TestMemoryAndRedis_Agree(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	mem, err := NewMemory(2, 2, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	red, err := NewRedis(client, 2, 2, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	mem.SetNowForTest(func() time.Time { return now })
	red.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 4; i++ {
		mAllowed, mLevel, _, mErr := mem.Take("k")
		if mErr != nil {
			t.Fatal(mErr)
		}
		rAllowed, rLevel, _, rErr := red.Take("k")
		if rErr != nil {
			t.Fatal(rErr)
		}
		if mAllowed != rAllowed {
			t.Fatalf("step %d allowed mem %v redis %v", i, mAllowed, rAllowed)
		}
		if waterClass(mLevel, 2) != waterClass(rLevel, 2) {
			t.Fatalf("step %d water mem %v redis %v", i, mLevel, rLevel)
		}
	}
	mLevel, err := mem.Level("k")
	if err != nil {
		t.Fatal(err)
	}
	rLevel, err := red.Level("k")
	if err != nil {
		t.Fatal(err)
	}
	if waterClass(mLevel, 2) != waterClass(rLevel, 2) {
		t.Fatalf("level water mem %v redis %v", mLevel, rLevel)
	}
}

func TestRedis_Unreachable(t *testing.T) {
	client := &simpleredis.SimpleRedis{}
	client.Init("127.0.0.1:1", "", "")
	limiter, err := NewRedis(client, 1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = limiter.Take("k")
	if err == nil || err.Error() != simpleredis.RedisUnreachable {
		t.Fatalf("want redis:unreachable, got %v", err)
	}
}

func TestClose_StopsTickerAndKeepsRedis(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 1, minSyncRate, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	limiter.Sleep()
	limiter.Close()
	if _, _, _, err := limiter.Take("k"); err != nil {
		t.Fatalf("add after close: %v", err)
	}
	limiter.Wake()
	limiter.mu.Lock()
	running := limiter.stop != nil
	limiter.mu.Unlock()
	if running {
		t.Fatal("Wake after Close started a ticker")
	}
	exact, err := NewRedis(client, 1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := exact.Take("still-open"); err != nil {
		t.Fatalf("redis closed by limiter: %v", err)
	}
}

func TestBuffered_TickerFlushesWithoutSleep(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 10, time.Millisecond, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limiter.Close() })
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if allowed, _, _, err := limiter.Take("tick-k"); err != nil || !allowed {
		t.Fatalf("first take: allowed %v err %v", allowed, err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		limiter.mu.Lock()
		state := limiter.keys["tick-k"]
		flushed := state != nil && state.localPours == 0 && state.redisWater == 1
		limiter.mu.Unlock()
		if flushed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("ticker did not flush local pours")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestNewRedis_ExactModeHasNoTicker(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := &simpleredis.SimpleRedis{}
	client.Init(addr, "", "")
	limiter, err := NewRedis(client, 1, 1, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	if limiter.stop != nil || limiter.ticker != nil {
		t.Fatal("exact mode started a ticker")
	}
}

// waterClass is empty, partial, or full.
func waterClass(water, capacity float64) int {
	const eps = 1e-6
	if water <= eps {
		return 0
	}
	if water >= capacity-eps {
		return 2
	}
	return 1
}
