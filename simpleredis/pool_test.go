package simpleredis

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestConnectionIsReused(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if fake.connections() != 0 {
		t.Fatalf("New opened %d connections, want 0", fake.connections())
	}

	for i := 0; i < 25; i++ {
		if _, err := redis.Get("hit"); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}
	if fake.connections() != 1 {
		t.Fatalf("25 sequential Get opened %d connections, want 1", fake.connections())
	}
}

func TestConcurrentCommandsStayWithinPool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := redis.Get("hit"); err != nil {
					t.Errorf("Get: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := fake.connections(); got > 8 {
		t.Fatalf("8 goroutines opened %d connections, want at most 8", got)
	}
}

func TestRejectedAuthIsReturned(t *testing.T) {
	replies := []string{
		"-NOAUTH Authentication required.\r\n",
		"-WRONGPASS invalid password\r\n",
		"-NOPERM this user has no permissions\r\n",
		"-ERR Client sent AUTH, but no password is set\r\n",
	}
	for _, reply := range replies {
		addr := startStaticRedis(t, reply)
		redis := New(Config{Host: addr})
		if _, err := redis.Get("a"); err == nil || err.Error() != RedisNoAuth {
			t.Fatalf("Get against %q = %v, want %s", reply, err, RedisNoAuth)
		}
	}
}

// TestStaleConnectionIsRetried proves client-fd close retries on SetDeadline/os.ErrClosed, not peer EOF.
func TestStaleConnectionIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Client-side close so SetDeadline fails with os.ErrClosed and ioError never sees io.EOF.
	redis.idleConnsMu.Lock()
	for _, conn := range redis.idleConns {
		conn.close()
	}
	redis.idleConnsMu.Unlock()

	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("Get on a client-closed pooled connection: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get = %q, want %q", got, "t")
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}

// TestPeerClosedIdleConnEOFIsRetried proves a server-closed idle socket is retried on a new dial.
func TestPeerClosedIdleConnEOFIsRetried(t *testing.T) {
	fake, addr := startPeerCloseFake(t, map[string]string{"hit": "t"}, true)
	redis := New(Config{Host: addr})

	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("first Get = %q, want t", got)
	}
	redis.idleConnsMu.Lock()
	if len(redis.idleConns) != 1 {
		redis.idleConnsMu.Unlock()
		t.Fatalf("idle after first Get = %d, want 1", len(redis.idleConns))
	}
	dead := redis.idleConns[0]
	redis.idleConnsMu.Unlock()

	fake.waitFirstClosed(t)

	got, err = redis.Get("hit")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("second Get = %q, want t", got)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
	redis.idleConnsMu.Lock()
	for _, conn := range redis.idleConns {
		if conn == dead {
			redis.idleConnsMu.Unlock()
			t.Fatal("dead conn still in idle")
		}
	}
	redis.idleConnsMu.Unlock()
}

// TestPeerClosedIdleRetryBorrowFailsUnreachable proves retry borrow after peer close returns redis:unreachable when the listener is gone.
func TestPeerClosedIdleRetryBorrowFailsUnreachable(t *testing.T) {
	fake, addr := startPeerCloseFake(t, map[string]string{"hit": "t"}, false)
	redis := New(Config{Host: addr})

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	fake.waitFirstClosed(t)

	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("second Get = %v, want %s", err, RedisUnreachable)
	}
}

func TestAuthAndSelectOncePerDial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr, Pass: "secret", Database: "2"})

	for i := 0; i < 3; i++ {
		if _, err := redis.Get("hit"); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}
	auths, selects, gets := fake.handshakeCounts()
	if fake.connections() != 1 {
		t.Fatalf("opened %d connections, want 1", fake.connections())
	}
	if auths != 1 || selects != 1 || gets != 3 {
		t.Fatalf("AUTH=%d SELECT=%d GET=%d, want 1, 1, 3", auths, selects, gets)
	}
}

func TestIdleTimeoutOpensANewConnection(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	redis.idleConnsMu.Lock()
	if len(redis.idleConns) != 1 {
		redis.idleConnsMu.Unlock()
		t.Fatalf("idle = %d, want 1", len(redis.idleConns))
	}
	redis.idleConns[0].lastUsed = time.Now().Add(-redis.IdleTimeout() - time.Second)
	redis.idleConnsMu.Unlock()

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("Get after idle timeout: %v", err)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}

func TestBurstGetsStayWithinLiveCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 500 * time.Microsecond
	fake.mu.Unlock()
	redis := New(Config{Host: addr})

	const bursts = 5
	const perBurst = 64
	var wg sync.WaitGroup
	for burst := 0; burst < bursts; burst++ {
		wg.Add(perBurst)
		for i := 0; i < perBurst; i++ {
			go func() {
				defer wg.Done()
				if _, err := redis.Get("hit"); err != nil {
					t.Errorf("Get: %v", err)
				}
			}()
		}
		wg.Wait()
	}
	if got := fake.connections(); got > defaultPoolSize {
		t.Fatalf("burst Get opened %d connections, want at most %d", got, defaultPoolSize)
	}
}

func TestOverlappingCallersDoNotDialPastLiveCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 20 * time.Millisecond
	fake.mu.Unlock()
	redis := New(Config{Host: addr})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := redis.Get("hit"); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := fake.connections(); got > defaultPoolSize {
		t.Fatalf("32 overlapping Get opened %d connections, want at most %d", got, defaultPoolSize)
	}
}

func TestPoolWaitTimesOutWithoutExtraDial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 300 * time.Millisecond
	fake.mu.Unlock()
	redis := New(Config{Host: addr, PoolSize: 2, PoolTimeout: 50 * time.Millisecond})

	started := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- struct{}{}
			if _, err := redis.Get("hit"); err != nil {
				t.Errorf("holder Get: %v", err)
			}
		}()
	}
	<-started
	<-started
	deadline := time.Now().Add(time.Second)
	for fake.connections() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("holders did not dial")
		}
		time.Sleep(time.Millisecond)
	}
	waitStarted := time.Now()
	_, err := redis.Get("hit")
	waited := time.Since(waitStarted)
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("waiter Get = %v, want %s", err, RedisUnreachable)
	}
	if waited >= 2*redis.PoolTimeout() {
		t.Fatalf("waiter Get took %v, want one PoolTimeout (not MaxRetries * PoolTimeout)", waited)
	}
	wg.Wait()
	if got := fake.connections(); got > 2 {
		t.Fatalf("timeout path opened %d connections, want at most 2", got)
	}
}

func TestReleaseKeepsSocketWhenLiveUnderCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 20 * time.Millisecond
	fake.mu.Unlock()
	redis := New(Config{Host: addr, PoolSize: 16})

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := redis.Get("hit"); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := len(redis.idleConns); got <= 8 {
		t.Fatalf("idle %d after 12 Gets with poolSize 16, want more than maxIdleConns kept", got)
	}
	if got := fake.connections(); got < 9 {
		t.Fatalf("12 overlapping Get opened %d connections, want at least 9 so idle can exceed maxIdleConns under poolSize 16", got)
	}
	if got := fake.connections(); got > 16 {
		t.Fatalf("opened %d connections, want at most 16", got)
	}
	before := fake.connections()
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("reuse Get: %v", err)
	}
	if got := fake.connections(); got != before {
		t.Fatalf("reuse Get dialed, connections %d -> %d", before, got)
	}
}

func TestTruncatedBulkIsUnreachableAndNotPooled(t *testing.T) {
	// ReadSlice takes the complete `$100` head; io.ReadFull then fails on the 40-byte payload.
	truncated := append([]byte("$100\r\n"), bytes.Repeat([]byte("x"), 40)...)
	ownValue := []byte("$5\r\nhello\r\n")
	addr := startRawReplyRedis(t, []rawReply{
		{payload: truncated, closeAfter: true},
		{payload: ownValue, closeAfter: true},
	})
	// MaxRetries off: default retry would redial and hide the truncated classification.
	redis := New(Config{Host: addr, MaxRetries: -1})

	_, err := redis.Get("k")
	if err == nil {
		t.Fatal("truncated Get: want error")
	}
	if err.Error() == RedisIssue {
		t.Fatalf("truncated Get = %v, must not be %s", err, RedisIssue)
	}
	if err.Error() != RedisUnreachable {
		t.Fatalf("truncated Get = %v, want %s", err, RedisUnreachable)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after truncated Get = %d, want 0", got)
	}

	got, err := redis.Get("k")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("second Get = %q, want %q", got, "hello")
	}
}

func TestMalformedBulkHeaderIsIssueAndNotPooled(t *testing.T) {
	addr := startStaticRedis(t, "$abc\r\n")
	redis := New(Config{Host: addr})

	_, err := redis.Get("k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("malformed bulk Get = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after malformed bulk = %d, want 0", got)
	}
}

func TestIllegalArrayElementHeadIsIssueAndNotPooled(t *testing.T) {
	addr := startStaticRedis(t, "*1\r\n#x\r\n")
	redis := New(Config{Host: addr})

	_, err := redis.Eval("return {1}", nil, nil)
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("illegal array head = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after illegal array head = %d, want 0", got)
	}
}
