package simpleredis

import (
	"bufio"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTimeoutOnReusedConnIsNotRetried(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var accepts int
	var mu sync.Mutex
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			accepts++
			mu.Unlock()
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				if _, err := readCommand(reader); err != nil {
					return
				}
				_, _ = io.WriteString(conn, "$1\r\nt\r\n")
				time.Sleep(3 * time.Second)
			}(conn)
		}
	}()

	redis := New(Config{Host: listener.Addr().String()})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisTimeout {
		t.Fatalf("second Get = %v, want %s", err, RedisTimeout)
	}
	mu.Lock()
	got := accepts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("opened %d connections, want 1 (timeout must not retry)", got)
	}
}

func TestLostReplyIncrIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	fake.armCloseBeforeReplyOnceForTest()
	gotIncr, err := redis.Incr("counter")
	if err != nil {
		t.Fatalf("Incr after close-before-reply: %v", err)
	}
	if gotIncr != 2 {
		t.Fatalf("Incr = %d, want 2 (double apply)", gotIncr)
	}
	if fake.incrCount() != 2 {
		t.Fatalf("INCR count = %d, want 2", fake.incrCount())
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}

	got, err := redis.Get("counter")
	if err != nil {
		t.Fatalf("Get after lost-reply Incr: %v", err)
	}
	if string(got) != "2" {
		t.Fatalf("stored = %q, want 2", got)
	}
}

func TestLostReplyIncrByIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	fake.armCloseBeforeReplyOnceForTest()
	gotIncr, err := redis.IncrBy("counter", 5)
	if err != nil {
		t.Fatalf("IncrBy after close-before-reply: %v", err)
	}
	if gotIncr != 10 {
		t.Fatalf("IncrBy = %d, want 10 (double apply)", gotIncr)
	}
	if fake.incrByCount() != 2 {
		t.Fatalf("INCRBY count = %d, want 2", fake.incrByCount())
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}

	got, err := redis.Get("counter")
	if err != nil {
		t.Fatalf("Get after lost-reply IncrBy: %v", err)
	}
	if string(got) != "10" {
		t.Fatalf("stored = %q, want 10", got)
	}
}

func TestLostReplyEvalIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	fake.armCloseBeforeReplyOnceForTest()
	values, err := redis.Eval(kongIncrbyExpireatScript, []string{"win"}, []string{"7", "1700000000"})
	if err != nil {
		t.Fatalf("Eval after close-before-reply: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "7" {
		t.Fatalf("Eval = %q, want [7] (EVALSHA miss does not apply; retry EVAL applies once)", values)
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 2 || eval != 1 {
		t.Fatalf("after lost-reply Eval EVALSHA=%d EVAL=%d, want 2, 1", evalSha, eval)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}

	got, err := redis.Get("win")
	if err != nil {
		t.Fatalf("Get after lost-reply Eval: %v", err)
	}
	if string(got) != "7" {
		t.Fatalf("stored = %q, want 7", got)
	}
}

func TestLostReplyIncrMaxRetriesOff(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr, MaxRetries: -1})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	fake.armCloseBeforeReplyOnceForTest()
	_, err := redis.Incr("counter")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Incr = %v, want %s", err, RedisUnreachable)
	}
	if fake.incrCount() != 1 {
		t.Fatalf("INCR count = %d, want 1", fake.incrCount())
	}
	if fake.connections() != 1 {
		t.Fatalf("opened %d connections, want 1", fake.connections())
	}

	got, err := redis.Get("counter")
	if err != nil {
		t.Fatalf("Get after lost-reply Incr: %v", err)
	}
	if string(got) != "1" {
		t.Fatalf("stored = %q, want 1", got)
	}
}

func TestLostReplyGetIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	fake.armCloseBeforeReplyOnceForTest()
	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("Get after close-before-reply: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get = %q, want t", got)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}

func TestLoadingReplyIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	fake.armErrorReplyOnceForTest("-LOADING Redis is loading the dataset in memory\r\n")
	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("Get after LOADING: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get = %q, want t", got)
	}
}

func TestTryAgainReplyIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	fake.armErrorReplyOnceForTest("-TRYAGAIN Try again later\r\n")
	got, err := redis.Incr("counter")
	if err != nil {
		t.Fatalf("Incr after TRYAGAIN: %v", err)
	}
	if got != 1 {
		t.Fatalf("Incr = %d, want 1 (TRYAGAIN does not apply)", got)
	}
	if fake.incrCount() != 1 {
		t.Fatalf("INCR count = %d, want 1", fake.incrCount())
	}
}

func TestRetryableRedisRepliesAreRetried(t *testing.T) {
	cases := []struct {
		name  string
		reply string
	}{
		{"READONLY", "-READONLY You can only write against a master\r\n"},
		{"MASTERDOWN", "-MASTERDOWN Link with MASTER is down\r\n"},
		{"CLUSTERDOWN", "-CLUSTERDOWN The cluster is down\r\n"},
		{"max-clients", "-ERR max number of clients reached\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
			redis := New(Config{Host: addr})
			fake.armErrorReplyOnceForTest(tc.reply)
			got, err := redis.Get("hit")
			if err != nil {
				t.Fatalf("Get after %s: %v", tc.name, err)
			}
			if string(got) != "t" {
				t.Fatalf("Get after %s = %q, want t", tc.name, got)
			}
		})
	}
}

func TestRetryBackoffRangeAndOff(t *testing.T) {
	minBackoff := 8 * time.Millisecond
	maxBackoff := 512 * time.Millisecond
	for i := 0; i < 20; i++ {
		got := retryBackoff(1, minBackoff, maxBackoff)
		if got < 8*time.Millisecond || got >= 24*time.Millisecond {
			t.Fatalf("retryBackoff(1, 8ms, 512ms) = %v, want in [8ms, 24ms)", got)
		}
	}

	_, minOff, maxOff := retryLimits(0, -1, -1)
	if minOff != 0 || maxOff != 0 {
		t.Fatalf("retryLimits min/max -1 = %v, %v, want 0, 0", minOff, maxOff)
	}
	if got := retryBackoff(1, minOff, maxOff); got != 0 {
		t.Fatalf("retryBackoff with min 0 = %v, want 0", got)
	}

	maxRetries, _, _ := retryLimits(0, 0, 0)
	if maxRetries != 3 {
		t.Fatalf("MaxRetries 0 = %d, want 3", maxRetries)
	}
	maxRetries, _, _ = retryLimits(-1, 0, 0)
	if maxRetries != 0 {
		t.Fatalf("MaxRetries -1 = %d, want 0", maxRetries)
	}
}

func TestShouldRetryPoolWaitIsFalse(t *testing.T) {
	if shouldRetry(errPoolWait) {
		t.Fatal("shouldRetry(errPoolWait) = true, want false so MaxRetries does not multiply PoolTimeout")
	}
	if shouldRetry(errNotFromNew) {
		t.Fatal("shouldRetry(errNotFromNew) = true, want false so MaxRetries does not sleep a client that did not come from New")
	}
	if !shouldRetry(errUnreachable) {
		t.Fatal("shouldRetry(errUnreachable) = false, want true")
	}
}
