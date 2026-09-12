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

// testFakeRedis is an in-process RESP server for limiter unit tests.
type testFakeRedis struct {
	mu         sync.Mutex
	store      map[string]string
	lastExpire []string
	getCalls   int
}

// startTestFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startTestFakeRedis(t *testing.T) (*testFakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fake := &testFakeRedis{store: map[string]string{}}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
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
		case "EXPIRE", "EXPIREAT":
			f.lastExpire = append([]string(nil), args...)
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
			} else {
				_, _ = io.WriteString(conn, ":0\r\n")
			}
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
		f.mu.Unlock()
	}
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
