package simpleredis

import (
	"context"
	"sync"
	"testing"
	"time"
)

// commandBudgetForTest is the same duration bindCommandDeadline binds, so tests can wait past it
// before the production helper exists and after it lands.
func commandBudgetForTest(sr *SimpleRedis) time.Duration {
	maxRetries, _, _ := retryLimits(sr.maxRetries, sr.minRetryBackoff, sr.maxRetryBackoff)
	return time.Duration(maxRetries+1) * (sr.DialTimeout() + sr.IOTimeout())
}

// TestAbandonedSocketsEventuallyClose is the leaked-fd ticket: after PoolSize recovered panics
// and one command budget, the abandoned sockets are closed, not only turns restored.
func TestAbandonedSocketsEventuallyClose(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	const poolSize = 2
	redis := New(Config{
		Host:        addr,
		PoolSize:    poolSize,
		PoolTimeout: 20 * time.Millisecond,
		DialTimeout: 20 * time.Millisecond,
		IOTimeout:   20 * time.Millisecond,
		MaxRetries:  -1,
	})
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}
	for i := 0; i < poolSize; i++ {
		if err := bugPanicAfterBorrow(redis); err != nil {
			t.Fatalf("borrow %d: %v", i, err)
		}
	}
	time.Sleep(commandBudgetForTest(redis) + 30*time.Millisecond)
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after abandoned panics: %v", err)
	}
	if got := redis.LostTurns(); got < int64(poolSize) {
		t.Fatalf("LostTurns = %d, want at least %d", got, poolSize)
	}
	fake.waitOpenSocketsEqual(t, 1)
}

// TestLiveCommandIsNotReclaimed is the dangerous failure: a command still in flight must keep
// its socket when another goroutine hits pool-wait recovery.
func TestLiveCommandIsNotReclaimed(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.holdGetsForTest(t)
	redis := New(Config{
		Host:        addr,
		PoolSize:    1,
		PoolTimeout: 40 * time.Millisecond,
		DialTimeout: 50 * time.Millisecond,
		IOTimeout:   time.Second,
		MaxRetries:  -1,
	})
	started := make(chan struct{})
	var holderErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		_, holderErr = redis.Get(context.Background(), "hit")
	}()
	<-started
	fake.waitHeldGets(t, 1)
	openBefore := fake.openSockets()
	closedBefore := redis.AbandonedClosed()
	_, err := redis.Get(context.Background(), "hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("waiter Get = %v, want %s", err, RedisUnreachable)
	}
	if !IsPoolWait(err) {
		t.Fatalf("waiter Get = %v, want pool wait", err)
	}
	if got := fake.openSockets(); got != openBefore {
		t.Fatalf("open sockets after recovery = %d, want %d (holder socket closed)", got, openBefore)
	}
	if got := redis.AbandonedClosed(); got != closedBefore {
		t.Fatalf("AbandonedClosed = %d, want %d (holder counted as abandoned)", got, closedBefore)
	}
	fake.releaseHeldGetsForTest()
	wg.Wait()
	if holderErr != nil {
		t.Fatalf("holder Get: %v", holderErr)
	}
}

// TestGapConnIsNotReclaimed covers the borrow-to-do and do-to-release windows.
func TestGapConnIsNotReclaimed(t *testing.T) {
	t.Run("borrowToDo", func(t *testing.T) {
		fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
		redis := New(Config{
			Host:        addr,
			PoolSize:    1,
			PoolTimeout: 30 * time.Millisecond,
			DialTimeout: 50 * time.Millisecond,
			IOTimeout:   200 * time.Millisecond,
			MaxRetries:  -1,
		})
		if _, err := redis.Get(context.Background(), "hit"); err != nil {
			t.Fatalf("warm Get: %v", err)
		}
		conn, err := redis.borrow(context.Background())
		if err != nil {
			t.Fatalf("borrow: %v", err)
		}
		openBefore := fake.openSockets()
		_, waitErr := redis.Get(context.Background(), "hit")
		if waitErr == nil || !IsPoolWait(waitErr) && waitErr.Error() != RedisUnreachable {
			// #66 may refill in this gap (heldSockets is 0) and the waiter may succeed.
			// Either outcome is fine as long as the held socket is not closed.
			_ = waitErr
		}
		if got := fake.openSockets(); got < openBefore {
			t.Fatalf("open sockets = %d, want at least %d (gap socket closed)", got, openBefore)
		}
		_, _, doErr := redis.do(context.Background(), conn, [][]byte{[]byte("GET"), []byte("hit")})
		redis.release(conn, doErr == nil)
		if doErr != nil {
			t.Fatalf("do on gap conn: %v", doErr)
		}
	})
	t.Run("doToRelease", func(t *testing.T) {
		fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
		redis := New(Config{
			Host:        addr,
			PoolSize:    1,
			PoolTimeout: 30 * time.Millisecond,
			DialTimeout: 50 * time.Millisecond,
			IOTimeout:   200 * time.Millisecond,
			MaxRetries:  -1,
		})
		conn, err := redis.borrow(context.Background())
		if err != nil {
			t.Fatalf("borrow: %v", err)
		}
		_, reusable, doErr := redis.doWithHeldSocket(context.Background(), conn, [][]byte{[]byte("GET"), []byte("hit")})
		if doErr != nil {
			redis.release(conn, false)
			t.Fatalf("doWithHeldSocket: %v", doErr)
		}
		openBefore := fake.openSockets()
		_, waitErr := redis.Get(context.Background(), "hit")
		_ = waitErr
		if got := fake.openSockets(); got < openBefore {
			t.Fatalf("open sockets = %d, want at least %d (gap socket closed)", got, openBefore)
		}
		redis.release(conn, reusable)
	})
}

// TestLostTurnsAndAbandonedClosedReportHonestly keeps the counters truthful around reclaim.
func TestLostTurnsAndAbandonedClosedReportHonestly(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{
		Host:        addr,
		PoolSize:    1,
		PoolTimeout: 20 * time.Millisecond,
		DialTimeout: 20 * time.Millisecond,
		IOTimeout:   20 * time.Millisecond,
		MaxRetries:  -1,
	})
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}
	if got := redis.LostTurns(); got != 0 {
		t.Fatalf("LostTurns after warm Get = %d, want 0", got)
	}
	if got := redis.AbandonedClosed(); got != 0 {
		t.Fatalf("AbandonedClosed after warm Get = %d, want 0", got)
	}
	if err := bugPanicAfterBorrow(redis); err != nil {
		t.Fatalf("borrow: %v", err)
	}
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after leak before budget: %v", err)
	}
	if got := redis.LostTurns(); got < 1 {
		t.Fatalf("LostTurns = %d, want at least 1", got)
	}
	if got := redis.AbandonedClosed(); got != 0 {
		t.Fatalf("AbandonedClosed before budget = %d, want 0", got)
	}
	time.Sleep(commandBudgetForTest(redis) + 30*time.Millisecond)
	if err := bugPanicAfterBorrow(redis); err != nil {
		t.Fatalf("second borrow: %v", err)
	}
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after budget: %v", err)
	}
	if got := redis.AbandonedClosed(); got < 1 {
		t.Fatalf("AbandonedClosed after budget = %d, want at least 1", got)
	}
}
