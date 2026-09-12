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
	hangups              int
	authReply            string
	selectReply          string
	handshake            []string
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
func startFakeRedis(t testing.TB, store map[string]string) (*fakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	fake := &fakeRedis{store: store, loadedScripts: make(map[string]string), authReply: statusOKReply, selectReply: statusOKReply}
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
			// Client closed the socket (handshake failure calls conn.close).
			f.mu.Lock()
			f.hangups++
			f.mu.Unlock()
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
		f.handshake = append(f.handshake, "AUTH")
		return f.authReply
	case "SELECT":
		f.selects++
		f.handshake = append(f.handshake, "SELECT")
		return f.selectReply
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

// setHandshakeReplies sets AUTH and SELECT RESP replies (full wire including CRLF).
func (f *fakeRedis) setHandshakeReplies(authReply, selectReply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authReply = authReply
	f.selectReply = selectReply
}

// hangupCount is how many times serve exited after a read error (peer close).
func (f *fakeRedis) hangupCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hangups
}

// waitHangups waits until serve has observed want peer closes, or fails the test.
func (f *fakeRedis) waitHangups(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.hangupCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("hangups = %d, want %d", f.hangupCount(), want)
}

// handshakeAuthBeforeSelect is true when AUTH was recorded before SELECT on this fake.
func (f *fakeRedis) handshakeAuthBeforeSelect() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	authAt, selectAt := -1, -1
	for i, cmd := range f.handshake {
		if cmd == "AUTH" && authAt < 0 {
			authAt = i
		}
		if cmd == "SELECT" && selectAt < 0 {
			selectAt = i
		}
	}
	return authAt >= 0 && selectAt > authAt
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
