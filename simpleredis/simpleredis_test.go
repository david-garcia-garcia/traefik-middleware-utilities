package simpleredis

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRedis is an in-process RESP server backed by a string map.
type fakeRedis struct {
	mu         sync.Mutex
	store      map[string]string
	conns      int
	auths      int
	selects    int
	gets       int
	lastSet    []string
	lastExpire []string
	lastEval   []string
}

// startFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startFakeRedis(t *testing.T, store map[string]string) (*fakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	fake := &fakeRedis{store: store}
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

// serve answers AUTH/SELECT/GET/MGET/SET on one accepted socket.
func (f *fakeRedis) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		switch args[0] {
		case "AUTH":
			f.auths++
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "SELECT":
			f.selects++
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "GET":
			f.gets++
			_, _ = io.WriteString(conn, bulk(f.store, args[1]))
		case "MGET":
			_, _ = fmt.Fprintf(conn, "*%d\r\n", len(args)-1)
			for _, name := range args[1:] {
				_, _ = io.WriteString(conn, bulk(f.store, name))
			}
		case "SET":
			f.store[args[1]] = args[2]
			f.lastSet = append([]string(nil), args...)
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "INCR":
			afterIncr, incrErr := incrementStored(f.store, args[1], 1)
			if incrErr != nil {
				_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
				break
			}
			_, _ = fmt.Fprintf(conn, ":%d\r\n", afterIncr)
		case "INCRBY":
			delta, convErr := strconv.ParseInt(args[2], 10, 64)
			if convErr != nil {
				_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
				break
			}
			afterIncr, incrErr := incrementStored(f.store, args[1], delta)
			if incrErr != nil {
				_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
				break
			}
			_, _ = fmt.Fprintf(conn, ":%d\r\n", afterIncr)
		case "EXPIRE", "EXPIREAT":
			f.lastExpire = append([]string(nil), args...)
			_, _ = io.WriteString(conn, ":1\r\n")
		case "EVAL":
			f.lastEval = append([]string(nil), args...)
			if args[1] == kongIncrbyExpireatScript && len(args) >= 6 {
				key := args[3]
				delta, convErr := strconv.ParseInt(args[4], 10, 64)
				if convErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				_, existed := f.store[key]
				n, incrErr := incrementStored(f.store, key, delta)
				if incrErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				if !existed {
					f.lastExpire = []string{"EXPIREAT", key, args[5]}
				}
				_, _ = fmt.Fprintf(conn, ":%d\r\n", n)
			} else {
				_, _ = io.WriteString(conn, ":0\r\n")
			}
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
		f.mu.Unlock()
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

// lastEvalCommand returns the last EVAL argv.
func (f *fakeRedis) lastEvalCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastEval...)
}

// handshakeCounts returns AUTH, SELECT, and GET commands seen on this fake.
func (f *fakeRedis) handshakeCounts() (auths, selects, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.auths, f.selects, f.gets
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
	if len(got) != 6 || got[0] != "EVAL" || got[1] != kongIncrbyExpireatScript || got[2] != "1" || got[3] != "win" || got[4] != "7" || got[5] != "1700000000" {
		t.Fatalf("Eval argv = %v", got)
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

// TestSessionSourceImports fails if a non-test session file imports unsafe, cgo, or a dotted path.
func TestSessionSourceImports(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		file, parseErr := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, imp := range file.Imports {
			path, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("unquote %s: %v", imp.Path.Value, unquoteErr)
			}
			if path == "unsafe" || path == "C" || strings.Contains(path, ".") {
				t.Fatalf("%s imports %q", name, path)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no session source files scanned")
	}
}

// TestSimpleRedisProbeUseUnsafeOff fails if the probe manifest or compose turns useUnsafe on.
func TestSimpleRedisProbeUseUnsafeOff(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("..", "e2e", "simpleredisprobe", ".traefik.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifestUseUnsafeTrue(string(manifest)) {
		t.Fatal("simpleredisprobe manifest useUnsafe is true")
	}

	compose, err := os.ReadFile(filepath.Join("..", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(compose), "\n") {
		if strings.Contains(line, "reclaimprobe") {
			continue
		}
		const needle = "simpleredisprobe.settings.useunsafe="
		lower := strings.ToLower(line)
		idx := strings.Index(lower, needle)
		if idx < 0 {
			continue
		}
		found = true
		value := strings.TrimSpace(line[idx+len(needle):])
		value = strings.Trim(value, `"'`)
		if strings.EqualFold(value, "true") {
			t.Fatal("compose simpleredisprobe.settings.useunsafe is true")
		}
	}
	if !found {
		t.Fatal("compose has no simpleredisprobe.settings.useunsafe assignment")
	}
}

// manifestUseUnsafeTrue reports whether a Traefik plugin manifest sets useUnsafe true.
func manifestUseUnsafeTrue(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lower := strings.ToLower(trimmed)
		const key = "useunsafe:"
		if !strings.HasPrefix(lower, key) {
			continue
		}
		value := strings.TrimSpace(trimmed[len(key):])
		value = strings.Trim(value, `"'`)
		return strings.EqualFold(value, "true")
	}
	return false
}
