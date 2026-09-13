package tokenbucket

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// testHash is last/tokens for one fake Redis key.
type testHash struct {
	last   string
	tokens string
}

// testFakeRedis is an in-process RESP server for limiter unit tests.
type testFakeRedis struct {
	mu        sync.Mutex
	hashes    map[string]*testHash
	lastEval  []string
	evalReply string
}

// startTestFakeRedis listens on a local TCP port and serves an in-process RESP map.
func startTestFakeRedis(t *testing.T) (*testFakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fake := &testFakeRedis{hashes: map[string]*testHash{}}
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

// serve answers AUTH/SELECT/EVALSHA/EVAL on one accepted socket.
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
		case "EVALSHA":
			// No script cache: NOSCRIPT so SimpleRedis Eval falls back to EVAL.
			_, _ = io.WriteString(conn, "-NOSCRIPT No matching script. Please use EVAL.\r\n")
		case "EVAL":
			f.lastEval = append([]string(nil), args...)
			if f.evalReply != "" {
				_, _ = io.WriteString(conn, f.evalReply)
				break
			}
			if len(args) < 9 || strings.TrimSpace(args[1]) != strings.TrimSpace(allowScript) {
				_, _ = io.WriteString(conn, "-ERR unknown script\r\n")
				break
			}
			key := args[3]
			limitPerMicro, limErr := strconv.ParseFloat(args[4], 64)
			burst, burstErr := strconv.ParseFloat(args[5], 64)
			nowMicro, nowErr := strconv.ParseInt(args[7], 10, 64)
			maxDelayMicro, delayErr := strconv.ParseInt(args[8], 10, 64)
			if limErr != nil || burstErr != nil || nowErr != nil || delayErr != nil {
				_, _ = io.WriteString(conn, "-ERR bad argv\r\n")
				break
			}
			// Missing hash is a full bucket like Lua empty HGETALL, then consumeOne.
			tokens := burst
			last := nowMicro
			if hash := f.hashes[key]; hash != nil {
				parsedLast, lastErr := strconv.ParseInt(hash.last, 10, 64)
				parsedTokens, tokErr := strconv.ParseFloat(hash.tokens, 64)
				if lastErr != nil || tokErr != nil {
					_, _ = io.WriteString(conn, "-ERR bad hash\r\n")
					break
				}
				last = parsedLast
				tokens = parsedTokens
			}
			tokens, last, waitMicro := consumeOne(tokens, last, limitPerMicro, burst, nowMicro, maxDelayMicro)
			f.hashes[key] = &testHash{last: strconv.FormatInt(last, 10), tokens: strconv.FormatFloat(tokens, 'f', -1, 64)}
			_, _ = io.WriteString(conn, arrayBulks("true", strconv.FormatFloat(waitMicro, 'f', -1, 64), strconv.FormatFloat(tokens, 'f', -1, 64)))
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
		f.mu.Unlock()
	}
}

// setEvalReply overrides the EVAL RESP body (empty restores the script).
func (f *testFakeRedis) setEvalReply(reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.evalReply = reply
}

// lastEvalCommand returns the last EVAL argv.
func (f *testFakeRedis) lastEvalCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastEval...)
}

// arrayBulks formats a RESP array of bulk strings.
func arrayBulks(values ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(values))
	for _, value := range values {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(value), value)
	}
	return b.String()
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
