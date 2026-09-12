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
	msetexSends          int
	rejectMSetEX         bool
	closeBeforeReplyOnce bool
	errorReplyOnce       string
	getDelay             time.Duration
	open                 int
	heldGets             int
	holdCh               chan struct{}
	releaseHoldOnce      sync.Once
	lastSet              []string
	lastExpire           []string
	lastEval             []string
	lastMSetEX           []string
}

// startFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startFakeRedis(t testing.TB, store map[string]string) (*fakeRedis, string) {
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
			fake.open++
			fake.mu.Unlock()
			go fake.serve(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// serve answers AUTH/SELECT/GET/MGET/SET/INCR/MSETEX/EVALSHA/EVAL on one accepted socket.
func (f *fakeRedis) serve(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		f.mu.Lock()
		f.open--
		f.mu.Unlock()
	}()
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
		if args[0] == "GET" {
			if f.holdCh != nil {
				ch := f.holdCh
				f.heldGets++
				f.mu.Unlock()
				<-ch
				f.mu.Lock()
				f.heldGets--
			} else if f.getDelay > 0 {
				delay := f.getDelay
				f.mu.Unlock()
				time.Sleep(delay)
				f.mu.Lock()
			}
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
		return integerOneReply
	case "MSETEX":
		f.msetexSends++
		f.lastMSetEX = append([]string(nil), args...)
		if f.rejectMSetEX {
			return "-ERR unknown command 'MSETEX'\r\n"
		}
		f.applyMSetEXArgs(args)
		return integerOneReply
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

// openSockets is how many accepted sockets are still open.
func (f *fakeRedis) openSockets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open
}

// holdGetsForTest blocks each GET until releaseHeldGetsForTest. Cleanup releases the hold.
func (f *fakeRedis) holdGetsForTest(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	f.holdCh = make(chan struct{})
	f.mu.Unlock()
	t.Cleanup(f.releaseHeldGetsForTest)
}

// releaseHeldGetsForTest lets every held GET write its reply. Safe to call more than once.
func (f *fakeRedis) releaseHeldGetsForTest() {
	f.releaseHoldOnce.Do(func() {
		f.mu.Lock()
		ch := f.holdCh
		f.holdCh = nil
		f.mu.Unlock()
		if ch != nil {
			close(ch)
		}
	})
}

// waitHeldGets waits until at least want GET commands are blocked on the hold.
func (f *fakeRedis) waitHeldGets(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		got := f.heldGets
		f.mu.Unlock()
		if got >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	f.mu.Lock()
	got := f.heldGets
	f.mu.Unlock()
	t.Fatalf("held Gets = %d, want >= %d", got, want)
}

// waitOpenSocketsEqual waits until still-open sockets equal want (excess closed, not leaked).
func (f *fakeRedis) waitOpenSocketsEqual(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		open := f.openSockets()
		if open == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("open sockets = %d, want %d (excess leaked)", open, want)
		}
		time.Sleep(time.Millisecond)
	}
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

// lastMSetEXCommand returns the last native MSETEX argv.
func (f *fakeRedis) lastMSetEXCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastMSetEX...)
}

// msetexSendCount is how many MSETEX commands the fake has seen.
func (f *fakeRedis) msetexSendCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.msetexSends
}

// setRejectMSetEX makes later MSETEX commands reply unknown-command.
func (f *fakeRedis) setRejectMSetEX() {
	f.mu.Lock()
	f.rejectMSetEX = true
	f.mu.Unlock()
}

// applyMSetEXArgs stores or deletes pairs from a native MSETEX argv. Caller holds mu.
func (f *fakeRedis) applyMSetEXArgs(args []string) {
	if len(args) < 4 {
		return
	}
	pairCount, err := strconv.Atoi(args[1])
	if err != nil || pairCount < 0 {
		return
	}
	tokenIndex := 2 + 2*pairCount
	if len(args) < tokenIndex+2 {
		return
	}
	names := make([]string, pairCount)
	values := make([]string, pairCount)
	for i := 0; i < pairCount; i++ {
		names[i] = args[2+2*i]
		values[i] = args[3+2*i]
	}
	f.applyMSetPairs(names, values, args[tokenIndex], args[tokenIndex+1])
}

// applyMSetEXEval stores or deletes pairs from the fallback EVAL or EVALSHA argv. Caller holds mu.
func (f *fakeRedis) applyMSetEXEval(args []string) {
	if len(args) < 5 {
		return
	}
	pairCount, err := strconv.Atoi(args[2])
	if err != nil || pairCount < 0 {
		return
	}
	tokenIndex := 3 + 2*pairCount
	if len(args) < tokenIndex+2 {
		return
	}
	names := make([]string, pairCount)
	values := make([]string, pairCount)
	for i := 0; i < pairCount; i++ {
		names[i] = args[3+i]
		values[i] = args[3+pairCount+i]
	}
	f.applyMSetPairs(names, values, args[tokenIndex], args[tokenIndex+1])
}

// applyMSetPairs stores values, or deletes on past EXAT. Caller holds mu.
func (f *fakeRedis) applyMSetPairs(names, values []string, token, ttlText string) {
	ttl, err := strconv.ParseInt(ttlText, 10, 64)
	pastExat := token == "EXAT" && err == nil && ttl < time.Now().Unix()
	for i, name := range names {
		if pastExat {
			delete(f.store, name)
			continue
		}
		if i < len(values) {
			f.store[name] = values[i]
		}
	}
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
	if script == msetexFallbackScript {
		f.applyMSetEXEval(argv)
		return integerOneReply
	}
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

// integerOneReply is a RESP integer 1 (EXPIRE/MSETEX success and the MSetEX Lua fallback).
const integerOneReply = ":1\r\n"

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

// peerCloseFake is an in-process RESP server that closes the accepted socket after the first reply.
type peerCloseFake struct {
	mu          sync.Mutex
	store       map[string]string
	accepts     int
	firstClosed chan struct{}
}

// startPeerCloseFake listens, answers the first command, then Close()s that accepted socket (not the client).
// When acceptRetry is true, later accepts are served until read error. When false, the listener is closed after the first accept so a retry dial fails.
func startPeerCloseFake(t *testing.T, store map[string]string, acceptRetry bool) (*peerCloseFake, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	fake := &peerCloseFake{
		store:       store,
		firstClosed: make(chan struct{}),
	}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		fake.mu.Lock()
		fake.accepts++
		fake.mu.Unlock()
		fake.replyOnceAndClose(conn)
		if !acceptRetry {
			_ = listener.Close()
			return
		}
		for {
			next, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			fake.mu.Lock()
			fake.accepts++
			fake.mu.Unlock()
			go fake.serveUntilReadError(next)
		}
	}()
	return fake, listener.Addr().String()
}

// connections is how many TCP accepts the peer-close fake has seen.
func (f *peerCloseFake) connections() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accepts
}

// waitFirstClosed waits until the first accepted socket has been closed from the server.
func (f *peerCloseFake) waitFirstClosed(t *testing.T) {
	t.Helper()
	select {
	case <-f.firstClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not close the accepted socket")
	}
}

// replyOnceAndClose answers one command then Close()s the accepted socket.
func (f *peerCloseFake) replyOnceAndClose(conn net.Conn) {
	defer close(f.firstClosed)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	args, err := readCommand(reader)
	if err != nil {
		return
	}
	f.writeGetReply(conn, args)
}

// serveUntilReadError answers GET commands on one accepted socket until the client goes away.
func (f *peerCloseFake) serveUntilReadError(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		f.writeGetReply(conn, args)
	}
}

// writeGetReply writes a GET bulk reply for args, or a miss when the command is not GET.
func (f *peerCloseFake) writeGetReply(conn net.Conn, args []string) {
	name := ""
	if len(args) >= 2 {
		name = args[1]
	}
	f.mu.Lock()
	reply := bulk(f.store, name)
	f.mu.Unlock()
	_, _ = io.WriteString(conn, reply)
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

// startSequentialRedis replies with replies[n] for the n-th command on each accepted socket.
func startSequentialRedis(t *testing.T, replies []string) string {
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
				n := 0
				for {
					if _, err := readCommand(reader); err != nil {
						return
					}
					if n >= len(replies) {
						return
					}
					_, _ = io.WriteString(conn, replies[n])
					n++
				}
			}(conn)
		}
	}()
	return listener.Addr().String()
}

// startWriteThenCloseRedis reads one command, writes a partial RESP reply, then closes the socket.
func startWriteThenCloseRedis(t *testing.T, partialReply string) string {
	t.Helper()
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
		reader := bufio.NewReader(conn)
		if _, err := readCommand(reader); err != nil {
			return
		}
		_, _ = io.WriteString(conn, partialReply)
	}()
	return listener.Addr().String()
}

// startRetryBorrowFailRedis accepts one connection, replies a GET hit, closes the listener, then writes a truncated reply on the reused socket so retry dial fails.
func startRetryBorrowFailRedis(t *testing.T, truncatedReply string) (addr string, listenerClosed <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	closed := make(chan struct{})

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		if _, err := readCommand(reader); err != nil {
			return
		}
		_, _ = io.WriteString(conn, "$1\r\nt\r\n")
		_ = listener.Close()
		close(closed)
		if _, err := readCommand(reader); err != nil {
			return
		}
		_, _ = io.WriteString(conn, truncatedReply)
		_ = conn.Close()
	}()
	return listener.Addr().String(), closed
}
