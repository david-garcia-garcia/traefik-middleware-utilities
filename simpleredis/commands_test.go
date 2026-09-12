package simpleredis

import (
	"bufio"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestGetHitAndMiss(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get hit = %q, want %q", got, "t")
	}

	if _, err = redis.Get("missing"); err == nil || err.Error() != RedisMiss {
		t.Fatalf("Get missing = %v, want %s", err, RedisMiss)
	}
}

func TestSetSendsExpire(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})
	if err := redis.Set("k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := fake.lastSetCommand()
	want := []string{"SET", "k", "v", "EX", "60"}
	if len(got) != len(want) {
		t.Fatalf("SET argv %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SET argv %q, want %q", got, want)
		}
	}
}

func TestMGetHitsMissesAndEmpty(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"a": "t", "c": "f"})
	redis := New(Config{Host: addr})

	got, err := redis.MGet([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("MGet: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("MGet returned %d values, want 3", len(got))
	}
	if string(got[0]) != "t" || got[1] != nil || string(got[2]) != "f" {
		t.Fatalf("MGet = %q, want [t <nil> f]", got)
	}

	empty, err := redis.MGet(nil)
	if empty != nil || err != nil {
		t.Fatalf("MGet(nil) = %v, %v, want nil, nil", empty, err)
	}
	if fake.connections() != 1 {
		t.Fatalf("MGet opened %d connections, want 1", fake.connections())
	}
}

func TestSetReturnsReplyError(t *testing.T) {
	addr := startStaticRedis(t, "-ERR value is not an integer or out of range\r\n")
	redis := New(Config{Host: addr})

	err := redis.Set("k", []byte("v"), -1)
	if err == nil {
		t.Fatal("Set swallowed the error reply")
	}
	if err.Error() != "ERR value is not an integer or out of range" {
		t.Fatalf("Set = %v", err)
	}
}

func TestDelSucceeds(t *testing.T) {
	addr := startStaticRedis(t, "+OK\r\n")
	redis := New(Config{Host: addr})

	if err := redis.Del("k"); err != nil {
		t.Fatalf("Del = %v", err)
	}
}

func TestDelIntegerReplySucceeds(t *testing.T) {
	addr := startStaticRedis(t, ":1\r\n")
	redis := New(Config{Host: addr})
	if err := redis.Del("k"); err != nil {
		t.Fatalf("Del against :1 = %v", err)
	}
}

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

func TestIncrMissingThenPresent(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	first, err := redis.Incr("counter")
	if err != nil {
		t.Fatalf("first Incr: %v", err)
	}
	if first != 1 {
		t.Fatalf("first Incr = %d, want 1", first)
	}
	second, err := redis.Incr("counter")
	if err != nil {
		t.Fatalf("second Incr: %v", err)
	}
	if second != 2 {
		t.Fatalf("second Incr = %d, want 2", second)
	}
}

func TestIncrByMissing(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	got, err := redis.IncrBy("counter", 5)
	if err != nil {
		t.Fatalf("IncrBy: %v", err)
	}
	if got != 5 {
		t.Fatalf("IncrBy = %d, want 5", got)
	}
}

func TestIncrNonIntegerValue(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"k": "abc"})
	redis := New(Config{Host: addr})

	_, err := redis.Incr("k")
	if err == nil {
		t.Fatal("Incr non-integer: want error")
	}
	if err.Error() == RedisIssue {
		t.Fatalf("Incr non-integer = %v, want Redis error text", err)
	}
}

func TestExpireAndExpireAtArgv(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"k": "1"})
	redis := New(Config{Host: addr})

	if err := redis.Expire("k", 60); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	got := fake.lastExpireCommand()
	if len(got) != 3 || got[0] != "EXPIRE" || got[1] != "k" || got[2] != "60" {
		t.Fatalf("Expire argv = %v, want EXPIRE k 60", got)
	}

	if err := redis.ExpireAt("k", 1700000000); err != nil {
		t.Fatalf("ExpireAt: %v", err)
	}
	got = fake.lastExpireCommand()
	if len(got) != 3 || got[0] != "EXPIREAT" || got[1] != "k" || got[2] != "1700000000" {
		t.Fatalf("ExpireAt argv = %v, want EXPIREAT k 1700000000", got)
	}
}

func TestExpireZeroReplyIsSuccess(t *testing.T) {
	addr := startStaticRedis(t, ":0\r\n")
	redis := New(Config{Host: addr})
	if err := redis.Expire("missing", 30); err != nil {
		t.Fatalf("Expire :0: %v", err)
	}
}

func TestEvalArgvAndIntegerReply(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	values, err := redis.Eval(kongIncrbyExpireatScript, []string{"win"}, []string{"7", "1700000000"})
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "7" {
		t.Fatalf("Eval = %q, want [7]", values)
	}
	got := fake.lastEvalCommand()
	if len(got) != 6 || got[0] != evalVerb || got[1] != kongIncrbyExpireatScript || got[2] != "1" || got[3] != "win" || got[4] != "7" || got[5] != "1700000000" {
		t.Fatalf("fallback EVAL argv = %v", got)
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 1 || eval != 1 {
		t.Fatalf("after first Eval EVALSHA=%d EVAL=%d, want 1, 1", evalSha, eval)
	}
}

func TestEvalLaterSendsEvalShaNotBody(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if _, err := redis.Eval(kongIncrbyExpireatScript, []string{"win"}, []string{"7", "1700000000"}); err != nil {
		t.Fatalf("first Eval: %v", err)
	}
	values, err := redis.Eval(kongIncrbyExpireatScript, []string{"win2"}, []string{"7", "1700000000"})
	if err != nil {
		t.Fatalf("second Eval: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "7" {
		t.Fatalf("second Eval = %q, want [7]", values)
	}
	got := fake.lastEvalCommand()
	digest := scriptSHA1Hex(kongIncrbyExpireatScript)
	if len(got) != 6 || got[0] != evalShaVerb || got[1] != digest || got[2] != "1" || got[3] != "win2" || got[4] != "7" || got[5] != "1700000000" {
		t.Fatalf("second argv = %v, want EVALSHA %s 1 win2 7 1700000000", got, digest)
	}
	for _, arg := range got {
		if arg == kongIncrbyExpireatScript {
			t.Fatalf("second argv included the script body: %v", got)
		}
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 2 || eval != 1 {
		t.Fatalf("after second Eval EVALSHA=%d EVAL=%d, want 2, 1", evalSha, eval)
	}
}

func TestEvalTwoScriptsTwoDigests(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if _, err := redis.Eval("return 1", nil, nil); err != nil {
		t.Fatalf("script A first: %v", err)
	}
	if _, err := redis.Eval("return 1", nil, nil); err != nil {
		t.Fatalf("script A second: %v", err)
	}
	argvA := fake.lastEvalCommand()
	if len(argvA) < 2 || argvA[0] != evalShaVerb {
		t.Fatalf("script A second argv = %v, want EVALSHA", argvA)
	}
	if _, err := redis.Eval("return 2", nil, nil); err != nil {
		t.Fatalf("script B first: %v", err)
	}
	if _, err := redis.Eval("return 2", nil, nil); err != nil {
		t.Fatalf("script B second: %v", err)
	}
	argvB := fake.lastEvalCommand()
	if len(argvB) < 2 || argvB[0] != evalShaVerb {
		t.Fatalf("script B second argv = %v, want EVALSHA", argvB)
	}
	if argvA[1] == argvB[1] {
		t.Fatalf("scripts A and B shared digest %q", argvA[1])
	}
}

func TestEvalEmptyKeysSendsNumkeysZero(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if _, err := redis.Eval("return 1", nil, nil); err != nil {
		t.Fatalf("first Eval: %v", err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 3 || got[0] != evalVerb || got[2] != "0" {
		t.Fatalf("fallback EVAL argv = %v, want EVAL … 0", got)
	}
	if _, err := redis.Eval("return 1", nil, nil); err != nil {
		t.Fatalf("second Eval: %v", err)
	}
	got = fake.lastEvalCommand()
	if len(got) < 3 || got[0] != evalShaVerb || got[2] != "0" {
		t.Fatalf("EVALSHA argv = %v, want EVALSHA … 0", got)
	}
}

func TestEvalEmptyKeys(t *testing.T) {
	addr := startStaticRedis(t, ":1\r\n")
	redis := New(Config{Host: addr})
	values, err := redis.Eval("return 1", nil, nil)
	if err != nil {
		t.Fatalf("Eval empty: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "1" {
		t.Fatalf("Eval empty = %q, want [1]", values)
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
	if !shouldRetry(errUnreachable) {
		t.Fatal("shouldRetry(errUnreachable) = false, want true")
	}
}
