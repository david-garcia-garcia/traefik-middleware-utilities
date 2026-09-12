package simpleredis

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// fakeRedis is an in-process RESP server backed by a string map.
type fakeRedis struct {
	mu                   sync.Mutex
	store                map[string]string
	loadedScripts        map[string]string
	conns                int
	auths                int
	selects              int
	gets                 int
	incrs                int
	incrBys              int
	evalShaCount         int
	evals                int
	closeBeforeReplyOnce bool
	errorReplyOnce       string
	getDelay             time.Duration
	lastSet              []string
	lastExpire           []string
	lastEval             []string
}

// startFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startFakeRedis(t *testing.T, store map[string]string) (*fakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	fake := &fakeRedis{store: store, loadedScripts: make(map[string]string)}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fake.mu.Lock()
			fake.conns++
			fake.mu.Unlock()
			go fake.serve(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// serve answers AUTH/SELECT/GET/MGET/SET/INCR/EVALSHA/EVAL on one accepted socket.
func (f *fakeRedis) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		if f.errorReplyOnce != "" {
			reply := f.errorReplyOnce
			f.errorReplyOnce = ""
			f.mu.Unlock()
			_, _ = io.WriteString(conn, reply)
			continue
		}
		if args[0] == "GET" && f.getDelay > 0 {
			delay := f.getDelay
			f.mu.Unlock()
			time.Sleep(delay)
			f.mu.Lock()
		}
		reply := f.commandReply(args)
		if f.closeBeforeReplyOnce {
			f.closeBeforeReplyOnce = false
			f.mu.Unlock()
			_ = conn.Close()
			return
		}
		_, _ = io.WriteString(conn, reply)
		f.mu.Unlock()
	}
}

// commandReply applies one command to the fake store and returns the RESP reply.
func (f *fakeRedis) commandReply(args []string) string {
	switch args[0] {
	case "AUTH":
		f.auths++
		return statusOKReply
	case "SELECT":
		f.selects++
		return statusOKReply
	case "GET":
		f.gets++
		return bulk(f.store, args[1])
	case "MGET":
		reply := fmt.Sprintf("*%d\r\n", len(args)-1)
		for _, name := range args[1:] {
			reply += bulk(f.store, name)
		}
		return reply
	case "SET":
		f.store[args[1]] = args[2]
		f.lastSet = append([]string(nil), args...)
		return statusOKReply
	case "INCR":
		f.incrs++
		afterIncr, incrErr := incrementStored(f.store, args[1], 1)
		if incrErr != nil {
			return incrementNotIntegerReply
		}
		return fmt.Sprintf(":%d\r\n", afterIncr)
	case "INCRBY":
		f.incrBys++
		delta, convErr := strconv.ParseInt(args[2], 10, 64)
		if convErr != nil {
			return incrementNotIntegerReply
		}
		afterIncr, incrErr := incrementStored(f.store, args[1], delta)
		if incrErr != nil {
			return incrementNotIntegerReply
		}
		return fmt.Sprintf(":%d\r\n", afterIncr)
	case "EXPIRE", "EXPIREAT":
		f.lastExpire = append([]string(nil), args...)
		return ":1\r\n"
	case evalShaVerb:
		f.evalShaCount++
		digest := args[1]
		script, loaded := f.loadedScripts[digest]
		if !loaded {
			f.lastEval = append([]string(nil), args...)
			return "-NOSCRIPT No matching script. Please use EVAL.\r\n"
		}
		return f.evalScriptReply(script, args)
	case evalVerb:
		f.evals++
		script := args[1]
		f.loadedScripts[scriptSHA1Hex(script)] = script
		return f.evalScriptReply(script, args)
	default:
		return statusOKReply
	}
}

// connections is how many TCP accepts the fake has seen.
func (f *fakeRedis) connections() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conns
}

// lastSetCommand returns the last SET argv (including EX and duration).
func (f *fakeRedis) lastSetCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastSet...)
}

// lastExpireCommand returns the last EXPIRE or EXPIREAT argv.
func (f *fakeRedis) lastExpireCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastExpire...)
}

// lastEvalCommand returns the last EVAL or EVALSHA argv.
func (f *fakeRedis) lastEvalCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastEval...)
}

// evalCommandCounts returns how many EVALSHA and EVAL commands the fake has seen.
func (f *fakeRedis) evalCommandCounts() (evalSha, eval int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.evalShaCount, f.evals
}

// evalScriptReply runs the Kong incrby+expireat path or replies :0. lastEval is that argv.
func (f *fakeRedis) evalScriptReply(script string, argv []string) string {
	f.lastEval = append([]string(nil), argv...)
	if script == kongIncrbyExpireatScript && len(argv) >= 6 {
		key := argv[3]
		delta, convErr := strconv.ParseInt(argv[4], 10, 64)
		if convErr != nil {
			return incrementNotIntegerReply
		}
		_, existed := f.store[key]
		n, incrErr := incrementStored(f.store, key, delta)
		if incrErr != nil {
			return incrementNotIntegerReply
		}
		if !existed {
			f.lastExpire = []string{"EXPIREAT", key, argv[5]}
		}
		return fmt.Sprintf(":%d\r\n", n)
	}
	return ":0\r\n"
}

// handshakeCounts returns AUTH, SELECT, and GET commands seen on this fake.
func (f *fakeRedis) handshakeCounts() (auths, selects, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.auths, f.selects, f.gets
}

// armCloseBeforeReplyOnceForTest makes the next command mutate then close without a reply.
func (f *fakeRedis) armCloseBeforeReplyOnceForTest() {
	f.mu.Lock()
	f.closeBeforeReplyOnce = true
	f.mu.Unlock()
}

// armErrorReplyOnceForTest writes reply once (LOADING/READONLY/MASTERDOWN/CLUSTERDOWN/TRYAGAIN/max-clients) without mutating the store.
func (f *fakeRedis) armErrorReplyOnceForTest(reply string) {
	f.mu.Lock()
	f.errorReplyOnce = reply
	f.mu.Unlock()
}

// incrCount is how many INCR commands the fake has seen.
func (f *fakeRedis) incrCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.incrs
}

// incrByCount is how many INCRBY commands the fake has seen.
func (f *fakeRedis) incrByCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.incrBys
}

// bulk formats a GET/MGET bulk string or a miss.
func bulk(store map[string]string, name string) string {
	value, found := store[name]
	if !found {
		return "$-1\r\n"
	}
	return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
}

// incrementStored adds delta to a decimal string slot, treating a missing key as 0.
func incrementStored(store map[string]string, name string, delta int64) (int64, error) {
	current := int64(0)
	if raw, found := store[name]; found {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, err
		}
		current = n
	}
	next := current + delta
	store[name] = strconv.FormatInt(next, 10)
	return next, nil
}

// kongIncrbyExpireatScript is the Kong flush snippet (KEYS declared, Lua 5.1-safe).
const kongIncrbyExpireatScript = `local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value`

// statusOKReply is a RESP simple-string OK.
const statusOKReply = "+OK\r\n"

// incrementNotIntegerReply is the Redis error when INCR/INCRBY cannot parse the stored value.
const incrementNotIntegerReply = "-ERR value is not an integer or out of range\r\n"

// readCommand parses one RESP array of bulk strings from the fake client.
func readCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(header[1 : len(header)-2])
	if err != nil {
		return nil, err
	}
	args := make([]string, count)
	for i := 0; i < count; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, length+2)
		if _, err = io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:length])
	}
	return args, nil
}

// startStaticRedis replies with the same canned RESP on every command.
func startStaticRedis(t *testing.T, reply string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					if _, err := readCommand(reader); err != nil {
						return
					}
					_, _ = io.WriteString(conn, reply)
				}
			}(conn)
		}
	}()
	return listener.Addr().String()
}

func TestGetHitAndMiss(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

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

func TestConnectionIsReused(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")
	if fake.connections() != 0 {
		t.Fatalf("Init opened %d connections, want 0", fake.connections())
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

func TestSetSendsExpire(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")
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

func TestConcurrentCommandsStayWithinPool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

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

func TestValueWithNewlinesSurvives(t *testing.T) {
	index := "10.0.0.0/8\n192.168.0.0/16\n172.16.0.0/12"
	_, addr := startFakeRedis(t, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if err := redis.Set("range-index", []byte(index), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := redis.Get("range-index")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != index {
		t.Fatalf("Get = %q, want %q", got, index)
	}
}

func TestMGetHitsMissesAndEmpty(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"a": "t", "c": "f"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

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

func TestMGetKeepsValuesWithNewlinesAligned(t *testing.T) {
	index := "10.0.0.0/8\n192.168.0.0/16"
	_, addr := startFakeRedis(t, map[string]string{"range-index": index, "a": "t", "c": "f"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	got, err := redis.MGet([]string{"range-index", "a", "c"})
	if err != nil {
		t.Fatalf("MGet: %v", err)
	}
	if string(got[0]) != index || string(got[1]) != "t" || string(got[2]) != "f" {
		t.Fatalf("MGet = %q, want [%q t f]", got, index)
	}
}

func TestMGetRejectsShortReply(t *testing.T) {
	addr := startStaticRedis(t, "*2\r\n$1\r\nt\r\n$1\r\nf\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if _, err := redis.MGet([]string{"a", "b", "c"}); err == nil || err.Error() != RedisIssue {
		t.Fatalf("MGet with 2 values for 3 keys = %v, want %s", err, RedisIssue)
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
		var redis SimpleRedis
		redis.Init(addr, "", "")
		if _, err := redis.Get("a"); err == nil || err.Error() != RedisNoAuth {
			t.Fatalf("Get against %q = %v, want %s", reply, err, RedisNoAuth)
		}
	}
}

func TestSetReturnsReplyError(t *testing.T) {
	addr := startStaticRedis(t, "-ERR value is not an integer or out of range\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if err := redis.Del("k"); err != nil {
		t.Fatalf("Del = %v", err)
	}
}

func TestDelIntegerReplySucceeds(t *testing.T) {
	addr := startStaticRedis(t, ":1\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")
	if err := redis.Del("k"); err != nil {
		t.Fatalf("Del against :1 = %v", err)
	}
}

func TestUnreachableHost(t *testing.T) {
	var redis SimpleRedis
	redis.Init("127.0.0.1:1", "", "")

	if _, err := redis.Get("a"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if _, err := redis.MGet([]string{"a"}); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("MGet = %v, want %s", err, RedisUnreachable)
	}
}

func TestStaleConnectionIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	redis.mu.Lock()
	for _, conn := range redis.idle {
		conn.close()
	}
	redis.mu.Unlock()

	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("Get on a dead pooled connection: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get = %q, want %q", got, "t")
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}

func TestCloseDrainsIdleAndDoesNotRepool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(redis.idle) != 1 {
		t.Fatalf("after Get idle = %d, want 1", len(redis.idle))
	}

	redis.Close()
	if len(redis.idle) != 0 {
		t.Fatalf("after Close idle = %d, want 0", len(redis.idle))
	}
	redis.Close()

	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if fake.connections() != 1 {
		t.Fatalf("Get after Close opened %d connections, want 1", fake.connections())
	}
	if len(redis.idle) != 0 {
		t.Fatalf("release after Close idle = %d, want 0", len(redis.idle))
	}
}

func TestClosedClientUnreachableIsNotRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	redis.Close()
	redis.MinRetryBackoff = 50 * time.Millisecond
	redis.MaxRetryBackoff = 50 * time.Millisecond

	started := time.Now()
	_, err := redis.Get("hit")
	elapsed := time.Since(started)
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if elapsed >= 50*time.Millisecond {
		t.Fatalf("Get after Close took %v, want no retry backoff", elapsed)
	}
	if fake.connections() != 1 {
		t.Fatalf("opened %d connections, want 1", fake.connections())
	}
}

func TestAuthAndSelectOncePerDial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "secret", "2")

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

	var redis SimpleRedis
	redis.Init(listener.Addr().String(), "", "")
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

func TestIoTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(3 * time.Second)
	}()

	var redis SimpleRedis
	redis.Init(listener.Addr().String(), "", "")
	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
}

func TestIdleTimeoutOpensANewConnection(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	redis.mu.Lock()
	if len(redis.idle) != 1 {
		redis.mu.Unlock()
		t.Fatalf("idle = %d, want 1", len(redis.idle))
	}
	redis.idle[0].lastUsed = time.Now().Add(-idleTimeout - time.Second)
	redis.mu.Unlock()

	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("Get after idle timeout: %v", err)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}

func TestIncrMissingThenPresent(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")
	if err := redis.Expire("missing", 30); err != nil {
		t.Fatalf("Expire :0: %v", err)
	}
}

func TestEvalArgvAndIntegerReply(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")
	values, err := redis.Eval("return 1", nil, nil)
	if err != nil {
		t.Fatalf("Eval empty: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "1" {
		t.Fatalf("Eval empty = %q, want [1]", values)
	}
}

func TestEvalMixedArrayReply(t *testing.T) {
	addr := startStaticRedis(t, "*3\r\n$3\r\nfoo\r\n:7\r\n+OK\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")
	values, err := redis.Eval("return {1}", nil, nil)
	if err != nil {
		t.Fatalf("Eval mixed: %v", err)
	}
	if len(values) != 3 || string(values[0]) != "foo" || string(values[1]) != "7" || string(values[2]) != "OK" {
		t.Fatalf("Eval mixed = %q", values)
	}
}

func TestEvalNestedArrayIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "*1\r\n*0\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")
	_, err := redis.Eval("return {{}}", nil, nil)
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Eval nested = %v, want %s", err, RedisIssue)
	}
}

func TestIncrGarbageIntegerPayload(t *testing.T) {
	addr := startStaticRedis(t, ":not-an-int\r\n")
	var redis SimpleRedis
	redis.Init(addr, "", "")
	_, err := redis.Incr("k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Incr garbage = %v, want %s", err, RedisIssue)
	}
}

func TestLostReplyIncrIsRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	var redis SimpleRedis
	redis.Init(addr, "", "")
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
	var redis SimpleRedis
	redis.Init(addr, "", "")
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
	var redis SimpleRedis
	redis.Init(addr, "", "")
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
	var redis SimpleRedis
	redis.MaxRetries = -1
	redis.Init(addr, "", "")
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
	var redis SimpleRedis
	redis.Init(addr, "", "")
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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
			var redis SimpleRedis
			redis.Init(addr, "", "")
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

func TestBurstGetsStayWithinLiveCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 500 * time.Microsecond
	fake.mu.Unlock()
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	if got := fake.connections(); got > poolSize {
		t.Fatalf("burst Get opened %d connections, want at most %d", got, poolSize)
	}
}

func TestOverlappingCallersDoNotDialPastLiveCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 20 * time.Millisecond
	fake.mu.Unlock()
	var redis SimpleRedis
	redis.Init(addr, "", "")

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
	if got := fake.connections(); got > poolSize {
		t.Fatalf("32 overlapping Get opened %d connections, want at most %d", got, poolSize)
	}
}

func TestPoolWaitTimesOutWithoutExtraDial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 300 * time.Millisecond
	fake.mu.Unlock()
	var redis SimpleRedis
	redis.poolSize = 2
	redis.poolTimeout = 50 * time.Millisecond
	redis.Init(addr, "", "")

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
	_, err := redis.Get("hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("waiter Get = %v, want %s", err, RedisUnreachable)
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
	var redis SimpleRedis
	redis.poolSize = 16
	redis.Init(addr, "", "")

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
	if got := len(redis.idle); got <= 8 {
		t.Fatalf("idle %d after 12 Gets with poolSize 16, want more than eight kept", got)
	}
	if got := fake.connections(); got < 9 {
		t.Fatalf("12 overlapping Get opened %d connections, want at least 9 so idle can exceed eight under poolSize 16", got)
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
