package windowcounter

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
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
}

// startTestFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startTestFakeRedis(t *testing.T) (*testFakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fake := &testFakeRedis{store: map[string]string{}, expireSec: map[string]int64{}, listener: listener}
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
		switch args[0] {
		case "AUTH", "SELECT":
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "GET":
			f.getCalls++
			_, _ = io.WriteString(conn, testBulk(f.store, args[1]))
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
			if args[1] == flushScript && len(args) >= 6 {
				key := args[3]
				delta, convErr := strconv.ParseInt(args[4], 10, 64)
				if convErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				_, existed := f.store[key]
				n, incrErr := incrementTestStore(f.store, key, delta)
				if incrErr != nil {
					_, _ = io.WriteString(conn, "-ERR value is not an integer or out of range\r\n")
					break
				}
				if !existed {
					f.lastExpire = []string{"EXPIREAT", key, args[5]}
				}
				_, _ = fmt.Fprintf(conn, ":%d\r\n", n)
			} else if args[1] == takeExactScript && len(args) >= 5 {
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
			} else {
				_, _ = io.WriteString(conn, ":0\r\n")
			}
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
		f.mu.Unlock()
	}
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
