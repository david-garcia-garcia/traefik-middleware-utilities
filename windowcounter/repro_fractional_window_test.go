package windowcounter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestRepro_FractionalWindowAccepted checks whether Take accepts a 1500ms window
// (not sub-second, not a whole number of seconds) and, if it does, whether Redis
// keys follow 1-second buckets while weight uses the 1.5s duration.
func TestRepro_FractionalWindowAccepted(t *testing.T) {
	fake, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, 0)
	if err != nil {
		t.Fatal(err)
	}

	const opaqueKey = "k"
	const window = 1500 * time.Millisecond
	now := time.Unix(1_700_000_001, 0)
	limiter.SetNowForTest(func() time.Time { return now })

	_, _, takeErr := limiter.Take(context.Background(), opaqueKey, 100, window)

	t.Run("A", func(t *testing.T) {
		if takeErr == nil {
			t.Fatal("want error")
		}
	})
	if takeErr != nil {
		return
	}

	t.Run("B", func(t *testing.T) {
		oneSecKey := redisWindowKey(opaqueKey, now.Unix()/1*1)
		twoSecKey := redisWindowKey(opaqueKey, now.Unix()/2*2)
		oneSecCount := redisCountOrZero(t, client, oneSecKey)
		twoSecCount := redisCountOrZero(t, client, twoSecKey)
		expireArgv := fake.lastExpireCommand()
		t.Logf("after first 1500ms Take: oneSecKey=%q count=%d twoSecKey=%q count=%d expire=%v",
			oneSecKey, oneSecCount, twoSecKey, twoSecCount, expireArgv)
		if oneSecKey == twoSecKey {
			t.Fatal("odd unix should make 1s and 2s keys differ")
		}
		if oneSecCount != 1 {
			t.Fatalf("1-second bucket %q count %d, want 1", oneSecKey, oneSecCount)
		}
		if twoSecCount != 0 {
			t.Fatalf("2-second bucket %q count %d, want 0 (keys should not use 2s alignment)", twoSecKey, twoSecCount)
		}
		if len(expireArgv) < 3 || expireArgv[0] != "EXPIRE" || expireArgv[2] != "2" {
			t.Fatalf("expire argv %v want EXPIRE … 2 (ttlSec=2*windowSec with windowSec=1)", expireArgv)
		}

		const fill int64 = 6
		for i := int64(1); i < fill; i++ {
			allowed, estimated, fillErr := limiter.Take(context.Background(), opaqueKey, 100, window)
			if fillErr != nil {
				t.Fatal(fillErr)
			}
			if !allowed {
				t.Fatalf("fill take %d denied, estimated %v", i+1, estimated)
			}
		}
		if got := redisCountOrZero(t, client, oneSecKey); got != fill {
			t.Fatalf("filled 1-second bucket %q count %d, want %d", oneSecKey, got, fill)
		}

		now = time.Unix(1_700_000_002, 0)
		allowed, estimated, takeErr := limiter.Take(context.Background(), opaqueKey, 100, window)
		if takeErr != nil {
			t.Fatal(takeErr)
		}
		if !allowed {
			t.Fatalf("take at +1s denied, estimated %v", estimated)
		}

		rolledOneSecKey := redisWindowKey(opaqueKey, now.Unix()/1*1)
		rolledTwoSecKey := redisWindowKey(opaqueKey, now.Unix()/2*2)
		current := redisCountOrZero(t, client, rolledOneSecKey)
		previous := redisCountOrZero(t, client, oneSecKey)
		elapsedOneSec := now.Unix() - now.Unix()/1*1
		oneSecFormula := float64(current) + float64(previous)*(1-float64(elapsedOneSec)/1)
		twoSecCurrent := redisCountOrZero(t, client, rolledTwoSecKey)
		twoSecPrevious := redisCountOrZero(t, client, twoSecKey)
		elapsedTwoSec := now.Unix() - now.Unix()/2*2
		twoSecFormula := float64(twoSecCurrent) + float64(twoSecPrevious)*(1-float64(elapsedTwoSec)/2)
		weightIfElapsedOneAndDenomWindow := 1 - float64(time.Second)/float64(window)
		estimateIfElapsedOneAndDenomWindow := float64(current) + float64(previous)*weightIfElapsedOneAndDenomWindow
		impliedWeight := (estimated - float64(current)) / float64(previous)

		t.Logf("at +1s (still inside 1.5s from fill): allowed=%v estimated=%v currentKey=%q current=%d previousKey=%q previous=%d elapsedOneSec=%d impliedWeight=%v",
			allowed, estimated, rolledOneSecKey, current, oneSecKey, previous, elapsedOneSec, impliedWeight)
		t.Logf("1s formula=%v 2s formula=%v estimate if elapsed=1s and denom=1.5s=%v (weight=%v)",
			oneSecFormula, twoSecFormula, estimateIfElapsedOneAndDenomWindow, weightIfElapsedOneAndDenomWindow)

		if current != 1 {
			t.Fatalf("rolled 1-second key %q count %d, want 1", rolledOneSecKey, current)
		}
		if previous != fill {
			t.Fatalf("previous 1-second bucket %q count %d, want %d", oneSecKey, previous, fill)
		}
		if estimated != oneSecFormula {
			t.Fatalf("estimated %v != 1s whole-second formula current+previous*(1-elapsed/1)=%v (weight denom 1.5s changed the number)", estimated, oneSecFormula)
		}
		if estimated == estimateIfElapsedOneAndDenomWindow {
			t.Fatalf("estimated %v used 1.5s denom with elapsed=1s; keys still rolled as 1s buckets", estimated)
		}
		if estimated != twoSecFormula {
			t.Fatalf("estimated %v matches 1s formula %v (implied weight %v, elapsed=%d, window=1.5s) not 2s whole-second formula %v; Redis rolled to %q after 1s",
				estimated, oneSecFormula, impliedWeight, elapsedOneSec, twoSecFormula, rolledOneSecKey)
		}
	})
}

// redisCountOrZero is the integer at key, or 0 on a Redis miss.
func redisCountOrZero(t *testing.T, client *simpleredis.SimpleRedis, key string) int64 {
	t.Helper()
	raw, err := client.Get(context.Background(), key)
	if simpleredis.IsMiss(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n, convErr := strconv.ParseInt(string(raw), 10, 64)
	if convErr != nil {
		t.Fatal(convErr)
	}
	return n
}
