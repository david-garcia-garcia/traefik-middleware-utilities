package simpleredis

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestValueWithNewlinesSurvives(t *testing.T) {
	index := "10.0.0.0/8\n192.168.0.0/16\n172.16.0.0/12"
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

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

func TestMGetKeepsValuesWithNewlinesAligned(t *testing.T) {
	index := "10.0.0.0/8\n192.168.0.0/16"
	_, addr := startFakeRedis(t, map[string]string{"range-index": index, "a": "t", "c": "f"})
	redis := New(Config{Host: addr})

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
	redis := New(Config{Host: addr})

	if _, err := redis.MGet([]string{"a", "b", "c"}); err == nil || err.Error() != RedisIssue {
		t.Fatalf("MGet with 2 values for 3 keys = %v, want %s", err, RedisIssue)
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

	redis := New(Config{Host: listener.Addr().String()})
	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
}

func TestEvalMixedArrayReply(t *testing.T) {
	addr := startStaticRedis(t, "*3\r\n$3\r\nfoo\r\n:7\r\n+OK\r\n")
	redis := New(Config{Host: addr})
	values, err := redis.Eval("return {1}", nil, nil)
	if err != nil {
		t.Fatalf("Eval mixed: %v", err)
	}
	if len(values) != 3 || string(values[0]) != "foo" || string(values[1]) != "7" || string(values[2]) != "OK" {
		t.Fatalf("Eval mixed = %q", values)
	}
}

func TestMalformedReplyIsIssueAndNotPooled(t *testing.T) {
	cases := []struct {
		name  string
		reply string
	}{
		{"http-shaped", "HTTP/1.1 400 Bad Request\r\n"},
		{"unknown-type", "?huh\r\n"},
		{"missing-cr", ":42\n"},
		{"empty-line", "\r\n"},
		{"unparseable-count", "*abc\r\n"},
		{"null-array", "*-1\r\n"},
		{"bad-element-type", "*1\r\n?bad\r\n"},
		{"empty-element-line", "*1\r\n\r\n"},
		{"nested-array", "*1\r\n*0\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := startStaticRedis(t, tc.reply)
			redis := New(Config{Host: addr})
			_, err := redis.Get("k")
			if err == nil || err.Error() != RedisIssue {
				t.Fatalf("Get = %v, want %s", err, RedisIssue)
			}
			if err.Error() == RedisMiss {
				t.Fatalf("Get = %v, must not be %s", err, RedisMiss)
			}
			if len(redis.idleConns) != 0 {
				t.Fatalf("idle = %d, want 0", len(redis.idleConns))
			}
		})
	}
}

func TestTruncatedReplyIsUnreachableAndNotPooled(t *testing.T) {
	cases := []struct {
		name         string
		partialReply string
	}{
		{"truncated-array", "*2\r\n$1\r\na\r\n"},
		{"truncated-bulk", "$10\r\nabc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := startWriteThenCloseRedis(t, tc.partialReply)
			redis := New(Config{Host: addr, MaxRetries: -1})
			_, err := redis.Get("k")
			if err == nil || err.Error() != RedisUnreachable {
				t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
			}
			if len(redis.idleConns) != 0 {
				t.Fatalf("idle = %d, want 0", len(redis.idleConns))
			}
		})
	}
}

func TestRetryBorrowFailsAfterDirtyReuse(t *testing.T) {
	addr, listenerClosed := startRetryBorrowFailRedis(t, "$10\r\nabc")
	redis := New(Config{Host: addr, MaxRetries: 1, MinRetryBackoff: -1})

	got, err := redis.Get("hit")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("first Get = %q, want t", got)
	}
	if len(redis.idleConns) != 1 {
		t.Fatalf("after first Get idle = %d, want 1", len(redis.idleConns))
	}
	select {
	case <-listenerClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not close after the pooled hit")
	}

	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("second Get = %v, want %s", err, RedisUnreachable)
	}
	if len(redis.idleConns) != 0 {
		t.Fatalf("idle = %d, want 0", len(redis.idleConns))
	}
}

func TestIncrGarbageIntegerPayload(t *testing.T) {
	addr := startStaticRedis(t, ":not-an-int\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Incr("k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Incr garbage = %v, want %s", err, RedisIssue)
	}
}

func TestReadBulkNonDollarHeadIsIssue(t *testing.T) {
	_, err := readBulk(bufio.NewReader(strings.NewReader("unused")), []byte(":1"))
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("readBulk non-$ head = %v, want %s", err, RedisIssue)
	}
}

func TestHeldIntegerSliceSurvivesLaterRead(t *testing.T) {
	addr := startSequentialRedis(t, []string{":111\r\n", ":222\r\n"})
	redis := New(Config{Host: addr})
	first, err := redis.Eval("return 111", nil, nil)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Eval: %v %q", err, first)
	}
	held := first[0]
	second, err := redis.Eval("return 222", nil, nil)
	if err != nil || len(second) != 1 || string(second[0]) != "222" {
		t.Fatalf("second Eval: %v %q", err, second)
	}
	if string(held) != "111" {
		t.Fatalf("held integer after later read = %q, want 111", held)
	}
}

func TestHeldStatusSliceSurvivesLaterRead(t *testing.T) {
	addr := startSequentialRedis(t, []string{"+FIRST\r\n", "+SECOND\r\n"})
	redis := New(Config{Host: addr})
	first, err := redis.Eval("return 'FIRST'", nil, nil)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Eval: %v %q", err, first)
	}
	held := first[0]
	second, err := redis.Eval("return 'SECOND'", nil, nil)
	if err != nil || len(second) != 1 || string(second[0]) != "SECOND" {
		t.Fatalf("second Eval: %v %q", err, second)
	}
	if string(held) != "FIRST" {
		t.Fatalf("held status after later read = %q, want FIRST", held)
	}
}

func TestArraySlotsSurviveLaterRead(t *testing.T) {
	addr := startSequentialRedis(t, []string{"*2\r\n:7\r\n+OK\r\n", ":1\r\n"})
	redis := New(Config{Host: addr})
	slots, err := redis.Eval("return {7, 'OK'}", nil, nil)
	if err != nil || len(slots) != 2 {
		t.Fatalf("array Eval: %v %q", err, slots)
	}
	heldInteger := slots[0]
	heldStatus := slots[1]
	later, err := redis.Eval("return 1", nil, nil)
	if err != nil || len(later) != 1 || string(later[0]) != "1" {
		t.Fatalf("later Eval: %v %q", err, later)
	}
	if string(heldInteger) != "7" || string(heldStatus) != "OK" {
		t.Fatalf("held array slots after later read = %q %q", heldInteger, heldStatus)
	}
}

// TestLongStatusLineIsIssueAndNotPooled proves a 5000-byte + status is redis:issue? and not pooled.
func TestLongStatusLineIsIssueAndNotPooled(t *testing.T) {
	payload := strings.Repeat("A", 5000)
	addr := startStaticRedis(t, "+"+payload+"\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Eval("return 'x'", nil, nil)
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("long status: %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after long status = %d, want 0", got)
	}
}

// testPeerCounter counts bytes a bufio.Reader pulled from a fake peer.
type testPeerCounter struct {
	body      io.Reader
	bytesRead int
}

// Read records how many bytes the bufio reader pulled from the fake peer.
func (peer *testPeerCounter) Read(p []byte) (int, error) {
	n, err := peer.body.Read(p)
	peer.bytesRead += n
	return n, err
}

// TestReadLineBufferFullIsIssue proves ErrBufferFull is redis:issue? and the peer delivered at most 4096 bytes.
func TestReadLineBufferFullIsIssue(t *testing.T) {
	// Unterminated stream: fill the 4096 buffer with no newline.
	unterminated := &testPeerCounter{body: bytes.NewReader(bytes.Repeat([]byte{'A'}, 8192))}
	line, err := readLine(bufio.NewReader(unterminated))
	if err != errIssue {
		t.Fatalf("unterminated = %v %q, want %v", err, line, errIssue)
	}
	if unterminated.bytesRead > 4096 {
		t.Fatalf("unterminated consumed %d, want <= 4096", unterminated.bytesRead)
	}

	// Terminated line longer than the buffer: still stop at ErrBufferFull.
	overCap := make([]byte, 0, 5003)
	overCap = append(overCap, '+')
	overCap = append(overCap, bytes.Repeat([]byte{'A'}, 5000)...)
	overCap = append(overCap, '\r', '\n')
	terminated := &testPeerCounter{body: bytes.NewReader(overCap)}
	line, err = readLine(bufio.NewReader(terminated))
	if err != errIssue {
		t.Fatalf("over-cap = %v %q, want %v", err, line, errIssue)
	}
	if terminated.bytesRead > 4096 {
		t.Fatalf("over-cap consumed %d, want <= 4096", terminated.bytesRead)
	}
}

// TestReadLineShortestLegalLines proves +OK, :1, and $-1 still decode.
func TestReadLineShortestLegalLines(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want string
	}{
		{"status-ok", "+OK\r\n", "+OK"},
		{"integer-one", ":1\r\n", ":1"},
		{"null-bulk", "$-1\r\n", "$-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readLine(bufio.NewReader(strings.NewReader(tc.wire)))
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("readLine = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGarbageBulkLengthIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "$abc\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Get("k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("garbage bulk = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after garbage bulk = %d, want 0", got)
	}
}

func TestGarbageArrayLengthIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "*\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Eval("return {}", nil, nil)
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("garbage array = %v, want %s", err, RedisIssue)
	}
}

func TestParseLen(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{in: "0", want: 0, wantOK: true},
		{in: "17", want: 17, wantOK: true},
		{in: "-1", want: -1, wantOK: true},
		{in: "", wantOK: false},
		{in: "-", wantOK: false},
		{in: "abc", wantOK: false},
		{in: "1a", wantOK: false},
		{in: "+1", wantOK: false},
		{in: "9999999999999999999999999999999999999999", wantOK: false},
	}
	for _, test := range tests {
		got, ok := parseLen([]byte(test.in))
		if ok != test.wantOK || (ok && got != test.want) {
			t.Fatalf("parseLen(%q) = %d, %v, want %d, %v", test.in, got, ok, test.want, test.wantOK)
		}
	}
}
