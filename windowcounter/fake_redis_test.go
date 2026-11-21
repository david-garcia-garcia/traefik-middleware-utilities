package windowcounter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// redisExpireCommand is the Redis EXPIRE verb the fake records for TTL assertions.
const redisExpireCommand = "EXPIRE"

// testFakeRedis is an in-process RESP server for limiter unit tests.
type testFakeRedis struct {
	mu                  sync.Mutex
	store               map[string]string
	expireSec           map[string]int64
	lastExpire          []string
	getCalls            int
	expireCommands      int
	expireFailRemaining int
	listener            net.Listener
	conns               []net.Conn

	getBlockPrefix string
	getHold        time.Duration
	getEntered     chan struct{}
	getEnteredOnce sync.Once
	getRelease     chan struct{}
	getReleaseOnce sync.Once

	// scriptCache makes EVAL store the script digest and EVALSHA of a stored digest run it (warm engine cache). Off: EVALSHA is always NOSCRIPT.
	scriptCache   bool
	loadedScripts map[string]string
	// commands counts every served command so benchmarks can report Redis round trips per operation.
	commands int

	// flushEvalKeys is the key count of every flush batch EVAL served, in order; lastFlushKeys is the last batch's KEYS.
	flushEvalKeys []int
	lastFlushKeys []string
	// incrbyFailPrefix and incrbyFailRemaining fail the INCRBY of the next N flushed keys with that prefix inside the batch.
	incrbyFailPrefix    string
	incrbyFailRemaining int
	// flushReply, when set, is the RESP reply of the next flush batch EVAL instead of running the script.
	flushReply string
	// flushHoldEntered and flushHoldRelease hold the next flush batch EVAL until released (see holdNextFlush).
	flushHoldEntered chan struct{}
	flushHoldRelease chan struct{}
}

// startTestFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startTestFakeRedis(tb testing.TB) (server *testFakeRedis, listenAddr string) {
	tb.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = listener.Close() })
	fake := &testFakeRedis{store: map[string]string{}, expireSec: map[string]int64{}, listener: listener, loadedScripts: map[string]string{}}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fake.mu.Lock()
			fake.conns = append(fake.conns, conn)
			fake.mu.Unlock()
			go fake.serve(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// serve answers AUTH/SELECT/GET/INCR/EXPIRE/EVALSHA/EVAL on one accepted socket.
func (f *testFakeRedis) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readTestCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.commands++
		// A cached EVALSHA runs the stored body as if it were EVAL; without the cache it is NOSCRIPT.
		if args[0] == "EVALSHA" && f.scriptCache && len(args) > 1 {
			if script, loaded := f.loadedScripts[args[1]]; loaded {
				args = append([]string{"EVAL", script}, args[2:]...)
			}
		}
		switch args[0] {
		case "AUTH", "SELECT":
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "GET":
			f.getCalls++
			redisKey := args[1]
			reply := testBulk(f.store, redisKey)
			timedHold := time.Duration(0)
			waitUntilRelease := false
			entered := f.getEntered
			release := f.getRelease
			if f.getBlockPrefix != "" && strings.HasPrefix(redisKey, f.getBlockPrefix) {
				timedHold = f.getHold
				waitUntilRelease = f.getHold == 0 && release != nil
				f.getBlockPrefix = ""
			}
			f.mu.Unlock()
			f.waitGetHold(timedHold, waitUntilRelease, entered, release)
			_, _ = io.WriteString(conn, reply)
			f.mu.Lock()
		case "INCR":
			afterIncr, incrErr := incrementTestStore(f.store, args[1], 1)
			if incrErr != nil {
				_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
				break
			}
			_, _ = fmt.Fprintf(conn, ":%d\r\n", afterIncr)
		case redisExpireCommand, "EXPIREAT":
			f.lastExpire = append([]string(nil), args...)
			if args[0] == redisExpireCommand {
				f.expireCommands++
				// Fail N EXPIRE commands with a non-retryable RESP error, then succeed.
				if f.expireFailRemaining > 0 {
					f.expireFailRemaining--
					_, _ = io.WriteString(conn, "-ERR expire failed\r\n")
					break
				}
				seconds, convErr := strconv.ParseInt(args[2], 10, 64)
				if convErr == nil {
					f.expireSec[args[1]] = seconds
				}
			}
			_, _ = io.WriteString(conn, ":1\r\n")
		case "EVALSHA":
			// No script cache: NOSCRIPT so SimpleRedis Eval falls back to EVAL.
			_, _ = io.WriteString(conn, "-NOSCRIPT No matching script. Please use EVAL.\r\n")
		case "EVAL":
			if f.scriptCache && len(args) > 1 {
				f.loadedScripts[simpleredis.ScriptSHA1Hex(args[1])] = args[1]
			}
			// break in an inner case leaves only this switch; nothing follows it in case "EVAL".
			switch {
			case args[1] == flushBatchScript:
				f.waitFlushHoldLocked()
				_, _ = io.WriteString(conn, f.flushBatchReplyLocked(args))
			case args[1] == takeExactScript && len(args) >= 5:
				key := args[3]
				ttl, convErr := strconv.ParseInt(args[4], 10, 64)
				if convErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				n, incrErr := incrementTestStore(f.store, key, 1)
				if incrErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				if pttlMsLocked(f.store, f.expireSec, key) < 0 {
					f.expireCommands++
					f.lastExpire = []string{redisExpireCommand, key, args[4]}
					if f.expireFailRemaining > 0 {
						f.expireFailRemaining--
						_, _ = io.WriteString(conn, "-ERR expire failed\r\n")
						break
					}
					f.expireSec[key] = ttl
				}
				_, _ = fmt.Fprintf(conn, ":%d\r\n", n)
			default:
				_, _ = io.WriteString(conn, ":0\r\n")
			}
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
		f.mu.Unlock()
	}
}

// enableScriptCache makes the fake keep EVAL digests so EVALSHA hits, like an engine with a warm script cache.
func (f *testFakeRedis) enableScriptCache() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scriptCache = true
}

// commandCount returns how many commands the fake has served.
func (f *testFakeRedis) commandCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.commands
}

// flushBatchReplyLocked runs flushBatchScript on EVAL argv (EVAL, script, numkeys, KEYS..., delta/expireAt pairs) and returns
// the RESP array: one integer per applied key, one "err:" bulk per key whose INCRBY failed. Caller holds f.mu.
func (f *testFakeRedis) flushBatchReplyLocked(args []string) string {
	numKeys, convErr := strconv.Atoi(args[2])
	if convErr != nil || len(args) != 3+3*numKeys {
		return "-ERR wrong number of arguments for flush batch\r\n"
	}
	keys := args[3 : 3+numKeys]
	argv := args[3+numKeys:]
	f.flushEvalKeys = append(f.flushEvalKeys, numKeys)
	f.lastFlushKeys = append([]string(nil), keys...)
	if f.flushReply != "" {
		reply := f.flushReply
		f.flushReply = ""
		return reply
	}
	var reply strings.Builder
	fmt.Fprintf(&reply, "*%d\r\n", numKeys)
	for i, key := range keys {
		n, incrErr := f.flushIncrByLocked(key, argv[2*i], argv[2*i+1])
		if incrErr != nil {
			slot := flushErrorSlotPrefix + incrErr.Error()
			fmt.Fprintf(&reply, "$%d\r\n%s\r\n", len(slot), slot)
			continue
		}
		fmt.Fprintf(&reply, ":%d\r\n", n)
	}
	return reply.String()
}

// flushIncrByLocked is one key of the batch script: INCRBY delta, then record EXPIREAT when the key did not exist. Caller holds f.mu.
func (f *testFakeRedis) flushIncrByLocked(key, rawDelta, rawExpireAt string) (int64, error) {
	if f.incrbyFailRemaining > 0 && strings.HasPrefix(key, f.incrbyFailPrefix) {
		f.incrbyFailRemaining--
		return 0, errors.New("ERR injected incrby failure")
	}
	delta, convErr := strconv.ParseInt(rawDelta, 10, 64)
	if convErr != nil {
		return 0, errors.New("ERR value is not an integer or out of range")
	}
	_, existed := f.store[key]
	n, incrErr := incrementTestStore(f.store, key, delta)
	if incrErr != nil {
		return 0, errors.New("ERR value is not an integer or out of range")
	}
	if !existed {
		f.lastExpire = []string{"EXPIREAT", key, rawExpireAt}
	}
	return n, nil
}

// failNextFlushIncrBy fails the INCRBY of the next n flushed keys whose Redis key has prefix; other keys in the batch apply.
func (f *testFakeRedis) failNextFlushIncrBy(prefix string, n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.incrbyFailPrefix = prefix
	f.incrbyFailRemaining = n
}

// replyNextFlush makes the next flush batch EVAL answer reply without applying anything.
func (f *testFakeRedis) replyNextFlush(reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushReply = reply
}

// holdNextFlush holds the next flush batch EVAL until release is called. entered closes once that EVAL is held.
func (f *testFakeRedis) holdNextFlush() (entered <-chan struct{}, release func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	enteredCh := make(chan struct{})
	releaseCh := make(chan struct{})
	f.flushHoldEntered = enteredCh
	f.flushHoldRelease = releaseCh
	var once sync.Once
	return enteredCh, func() { once.Do(func() { close(releaseCh) }) }
}

// waitFlushHoldLocked waits out a holdNextFlush with f.mu released, so other connections are served meanwhile. Caller holds f.mu.
func (f *testFakeRedis) waitFlushHoldLocked() {
	entered, release := f.flushHoldEntered, f.flushHoldRelease
	if release == nil {
		return
	}
	f.flushHoldEntered, f.flushHoldRelease = nil, nil
	f.mu.Unlock()
	close(entered)
	<-release
	f.mu.Lock()
}

// flushEvalKeyCounts returns the key count of every flush batch EVAL served so far.
func (f *testFakeRedis) flushEvalKeyCounts() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.flushEvalKeys...)
}

// lastFlushBatchKeys returns the KEYS of the last flush batch EVAL.
func (f *testFakeRedis) lastFlushBatchKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastFlushKeys...)
}

// failNextExpireCommands replies -ERR to the next n EXPIRE commands, then succeeds.
func (f *testFakeRedis) failNextExpireCommands(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expireFailRemaining = n
}

// expireCommandCount returns how many EXPIRE commands the fake has received.
func (f *testFakeRedis) expireCommandCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.expireCommands
}

// lastExpireCommand returns the last EXPIRE or EXPIREAT argv.
func (f *testFakeRedis) lastExpireCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastExpire...)
}

// getCallCount returns how many GET commands the fake has served.
func (f *testFakeRedis) getCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCalls
}

// blockGetPrefix holds the next GET whose Redis key has prefix until unblockGet or hold elapses.
func (f *testFakeRedis) blockGetPrefix(prefix string, hold time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getBlockPrefix = prefix
	f.getHold = hold
	f.getEntered = make(chan struct{})
	f.getRelease = make(chan struct{})
}

// waitGetBlocked waits until a matching GET has entered its hold, or timeout.
func (f *testFakeRedis) waitGetBlocked(timeout time.Duration) bool {
	f.mu.Lock()
	entered := f.getEntered
	f.mu.Unlock()
	if entered == nil {
		return false
	}
	select {
	case <-entered:
		return true
	case <-time.After(timeout):
		return false
	}
}

// unblockGet releases a GET waiting in blockGetPrefix so the fake does not hang past Close.
func (f *testFakeRedis) unblockGet() {
	f.getReleaseOnce.Do(func() {
		f.mu.Lock()
		release := f.getRelease
		f.mu.Unlock()
		if release != nil {
			close(release)
		}
	})
}

// Kill closes the listener and every accepted socket so later commands fail as unreachable.
func (f *testFakeRedis) Kill() {
	f.mu.Lock()
	listener := f.listener
	conns := append([]net.Conn(nil), f.conns...)
	f.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
}

// testBulk formats a GET/MGET bulk string or a miss.
func testBulk(store map[string]string, name string) string {
	value, found := store[name]
	if !found {
		return "$-1\r\n"
	}
	return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
}

// incrementTestStore adds delta to a decimal string slot, treating a missing key as 0.
func incrementTestStore(store map[string]string, name string, delta int64) (int64, error) {
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

// pttlMsLocked is Redis PTTL milliseconds: -2 missing, -1 exists with no TTL, else remaining ms from expireSec.
func pttlMsLocked(store map[string]string, expireSec map[string]int64, name string) int64 {
	if _, found := store[name]; !found {
		return -2
	}
	seconds, hasTTL := expireSec[name]
	if !hasTTL {
		return -1
	}
	return seconds * 1000
}

// readTestCommand parses one RESP array of bulk strings from the fake client.
func readTestCommand(reader *bufio.Reader) ([]string, error) {
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

// waitGetHold waits out a timed GET hold or until unblockGet. Caller has released f.mu so other keys are not stalled.
func (f *testFakeRedis) waitGetHold(timedHold time.Duration, waitUntilRelease bool, entered, release chan struct{}) {
	if timedHold == 0 && !waitUntilRelease {
		return
	}
	f.getEnteredOnce.Do(func() {
		if entered != nil {
			close(entered)
		}
	})
	if waitUntilRelease {
		<-release
		return
	}
	timer := time.NewTimer(timedHold)
	if release != nil {
		select {
		case <-release:
			timer.Stop()
		case <-timer.C:
		}
		return
	}
	<-timer.C
}
