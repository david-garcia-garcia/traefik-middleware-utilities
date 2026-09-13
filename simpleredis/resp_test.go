package simpleredis

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"math"
	"net"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValueWithNewlinesSurvives(t *testing.T) {
	index := "10.0.0.0/8\n192.168.0.0/16\n172.16.0.0/12"
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if err := redis.Set(context.Background(), "range-index", []byte(index), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := redis.Get(context.Background(), "range-index")
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

	got, err := redis.MGet(context.Background(), []string{"range-index", "a", "c"})
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

	if _, err := redis.MGet(context.Background(), []string{"a", "b", "c"}); err == nil || err.Error() != RedisIssue {
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
	if _, err := redis.Get(context.Background(), "hit"); err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
}

func TestEvalMixedArrayReply(t *testing.T) {
	addr := startStaticRedis(t, "*3\r\n$3\r\nfoo\r\n:7\r\n+OK\r\n")
	redis := New(Config{Host: addr})
	values, err := redis.Eval(context.Background(), "return {1}", ScriptSHA1Hex("return {1}"), nil, nil)
	if err != nil {
		t.Fatalf("Eval mixed: %v", err)
	}
	if len(values) != 3 || string(values[0]) != "foo" || string(values[1]) != "7" || string(values[2]) != "OK" {
		t.Fatalf("Eval mixed = %q", values)
	}
}

func TestReadReplyUnsupportedAndMalformed(t *testing.T) {
	cases := []struct {
		name    string
		wire    string
		errText string
		clean   bool
		slots   []string
	}{
		{name: "null-array", wire: "*-1\r\n", errText: RedisIssue, clean: false},
		{name: "nested-array", wire: "*1\r\n*1\r\n$1\r\na\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "error-in-array", wire: "*1\r\n-ERR nope\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "http-shaped", wire: "HTTP/1.1 400 Bad Request\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "unknown-type", wire: "?huh\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-null", wire: "_\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-bool", wire: "#t\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-double", wire: ",1.5\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-bignum", wire: "(1\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-map", wire: "%0\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-set", wire: "~0\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-verbatim", wire: "=0\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "resp3-push", wire: ">0\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "missing-cr", wire: ":42\n", errText: RedisIssue, clean: false},
		{name: "empty-line", wire: "\r\n", errText: RedisIssue, clean: false},
		{name: "unparseable-count", wire: "*abc\r\n", errText: RedisIssue, clean: false},
		{name: "empty-element-line", wire: "*1\r\n\r\n", errText: RedisIssue, clean: false},
		{name: "bad-element-type", wire: "*1\r\n?bad\r\n", errText: RedisUnsupportedReply, clean: false},
		{name: "tokenbucket-three-bulk", wire: "*3\r\n$4\r\ntrue\r\n$1\r\n0\r\n$1\r\n0\r\n", clean: true, slots: []string{"true", "0", "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values, clean, err := readReply(bufio.NewReader(strings.NewReader(tc.wire)))
			if clean != tc.clean {
				t.Fatalf("clean = %v, want %v (err=%v)", clean, tc.clean, err)
			}
			if tc.errText == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				if len(values) != len(tc.slots) {
					t.Fatalf("values = %q, want %q", values, tc.slots)
				}
				for i, slot := range tc.slots {
					if string(values[i]) != slot {
						t.Fatalf("values[%d] = %q, want %s", i, values[i], slot)
					}
				}
				return
			}
			if err == nil || err.Error() != tc.errText {
				t.Fatalf("err = %v, want %s", err, tc.errText)
			}
		})
	}
}

func TestMalformedReplyIsIssueAndNotPooled(t *testing.T) {
	cases := []struct {
		name  string
		reply string
	}{
		{"missing-cr", ":42\n"},
		{"empty-line", "\r\n"},
		{"unparseable-count", "*abc\r\n"},
		{"null-array", "*-1\r\n"},
		{"empty-element-line", "*1\r\n\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := startStaticRedis(t, tc.reply)
			redis := New(Config{Host: addr})
			_, err := redis.Get(context.Background(), "k")
			if err == nil || err.Error() != RedisIssue {
				t.Fatalf("Get = %v, want %s", err, RedisIssue)
			}
			if err.Error() == RedisMiss {
				t.Fatalf("Get = %v, must not be %s", err, RedisMiss)
			}
			if pooledIdle(redis) != 0 {
				t.Fatalf("idle = %d, want 0", pooledIdle(redis))
			}
		})
	}
}

func TestUnsupportedReplyIsNotPooled(t *testing.T) {
	cases := []struct {
		name  string
		reply string
	}{
		{"http-shaped", "HTTP/1.1 400 Bad Request\r\n"},
		{"unknown-type", "?huh\r\n"},
		{"bad-element-type", "*1\r\n?bad\r\n"},
		{"nested-array", "*1\r\n*0\r\n"},
		{"error-in-array", "*1\r\n-ERR nope\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := startStaticRedis(t, tc.reply)
			redis := New(Config{Host: addr})
			_, err := redis.Get(context.Background(), "k")
			if err == nil || err.Error() != RedisUnsupportedReply {
				t.Fatalf("Get = %v, want %s", err, RedisUnsupportedReply)
			}
			if err.Error() == RedisIssue || err.Error() == RedisUnreachable {
				t.Fatalf("Get = %v, must not be issue or unreachable", err)
			}
			if pooledIdle(redis) != 0 {
				t.Fatalf("idle = %d, want 0", pooledIdle(redis))
			}
		})
	}
}

func TestEvalNestedArrayRedials(t *testing.T) {
	addr, accepts := startFirstAcceptThenRestRedis(t, "*1\r\n*1\r\n$1\r\na\r\n", "*3\r\n$4\r\ntrue\r\n$1\r\n0\r\n$1\r\n0\r\n")
	redis := New(Config{Host: addr, MaxRetries: -1})
	_, err := redis.Eval(context.Background(), "return {1,{2}}", ScriptSHA1Hex("return {1,{2}}"), nil, nil)
	if err == nil || err.Error() != RedisUnsupportedReply {
		t.Fatalf("Eval nested = %v, want %s", err, RedisUnsupportedReply)
	}
	if err.Error() == RedisUnreachable {
		t.Fatalf("Eval nested = %v, must not be %s", err, RedisUnreachable)
	}
	if pooledIdle(redis) != 0 {
		t.Fatalf("idle after nested = %d, want 0", pooledIdle(redis))
	}
	if got := accepts(); got != 1 {
		t.Fatalf("accepts after nested Eval = %d, want 1", got)
	}
	values, err := redis.Eval(context.Background(), "return {tostring(true), tostring(0), tostring(0)}", ScriptSHA1Hex("return {tostring(true), tostring(0), tostring(0)}"), nil, nil)
	if err != nil {
		t.Fatalf("Eval after redial: %v", err)
	}
	if len(values) != 3 || string(values[0]) != "true" || string(values[1]) != "0" || string(values[2]) != "0" {
		t.Fatalf("Eval after redial = %q", values)
	}
	if got := accepts(); got != 2 {
		t.Fatalf("accepts after second Eval = %d, want 2 (redial)", got)
	}
}

func TestEvalTokenBucketThreeBulkStrings(t *testing.T) {
	addr := startStaticRedis(t, "*3\r\n$4\r\ntrue\r\n$1\r\n0\r\n$1\r\n0\r\n")
	redis := New(Config{Host: addr})
	values, err := redis.Eval(context.Background(), "return {tostring(true), tostring(0), tostring(0)}", ScriptSHA1Hex("return {tostring(true), tostring(0), tostring(0)}"), nil, nil)
	if err != nil {
		t.Fatalf("Eval three bulk: %v", err)
	}
	if len(values) != 3 || string(values[0]) != "true" || string(values[1]) != "0" || string(values[2]) != "0" {
		t.Fatalf("Eval three bulk = %q", values)
	}
}

func TestDesyncedSocketDoesNotServePreviousReplies(t *testing.T) {
	_, addr := startStrayExtraReplyFake(t, 5)
	redis := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})

	const commands = 12
	for i := 0; i < commands; i++ {
		key := "k" + strconv.Itoa(i)
		want := "v" + strconv.Itoa(i)
		got, err := redis.Get(context.Background(), key)
		if err != nil {
			continue
		}
		if string(got) != want {
			t.Fatalf("Get(%s) = %q, want %q or an error (command %d)", key, got, want, i)
		}
	}
}

func TestStrayExtraReplyIsNotPooled(t *testing.T) {
	fake, addr := startStrayExtraReplyFake(t, 5)
	redis := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})

	for i := 0; i < 5; i++ {
		key := "k" + strconv.Itoa(i)
		got, err := redis.Get(context.Background(), key)
		if err != nil {
			t.Fatalf("Get(%s): %v", key, err)
		}
		if string(got) != "v"+strconv.Itoa(i) {
			t.Fatalf("Get(%s) = %q, want v%d", key, got, i)
		}
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after stray extra = %d, want 0", got)
	}
	if fake.connections() != 1 {
		t.Fatalf("accepts after stray extra = %d, want 1", fake.connections())
	}
	if got := redis.OverFrees(); got != 0 {
		t.Fatalf("OverFrees after leftover destroy = %d, want 0", got)
	}

	got, err := redis.Get(context.Background(), "k5")
	if err != nil {
		t.Fatalf("Get(k5): %v", err)
	}
	if string(got) != "v5" {
		t.Fatalf("Get(k5) = %q, want v5", got)
	}
	if fake.connections() != 2 {
		t.Fatalf("accepts after next Get = %d, want 2", fake.connections())
	}
}

func TestAuthLeftoverIsNotParsedAsSelect(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("+OK\r\n$5\r\nSTRAY\r\n", statusOKReply)
	redis := New(Config{Host: addr, Pass: "secret", Database: "2", MaxRetries: -1})
	_, err := redis.Get(context.Background(), "hit")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Get = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle = %d, want 0", got)
	}
	auths, selects, gets := fake.handshakeCounts()
	if auths != 1 || selects != 0 || gets != 0 {
		t.Fatalf("AUTH=%d SELECT=%d GET=%d, want 1, 0, 0", auths, selects, gets)
	}
	if got := redis.OverFrees(); got != 0 {
		t.Fatalf("OverFrees after AUTH leftover = %d, want 0", got)
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
			_, err := redis.Get(context.Background(), "k")
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

	got, err := redis.Get(context.Background(), "hit")
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

	if _, err := redis.Get(context.Background(), "hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("second Get = %v, want %s", err, RedisUnreachable)
	}
	if len(redis.idleConns) != 0 {
		t.Fatalf("idle = %d, want 0", len(redis.idleConns))
	}
}

func TestIncrGarbageIntegerPayload(t *testing.T) {
	addr := startStaticRedis(t, ":not-an-int\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Incr(context.Background(), "k")
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

// TestReadBulkWrongTrailerIsIssue covers swapped CRLF, payload bytes as trailer, and empty bulk `$0`.
func TestReadBulkWrongTrailerIsIssue(t *testing.T) {
	cases := []struct {
		name   string
		head   []byte
		body   string
		want   string
		errTxt string
	}{
		{name: "swapped-crlf", head: []byte("$5"), body: "hello\n\r", errTxt: RedisIssue},
		{name: "payload-as-trailer", head: []byte("$5"), body: "helloXY", errTxt: RedisIssue},
		{name: "empty-ok", head: []byte("$0"), body: "\r\n", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := readBulk(bufio.NewReader(strings.NewReader(tc.body)), tc.head)
			if tc.errTxt != "" {
				if err == nil || err.Error() != tc.errTxt {
					t.Fatalf("readBulk = %q %v, want %s", data, err, tc.errTxt)
				}
				return
			}
			if err != nil {
				t.Fatalf("readBulk = %v", err)
			}
			if string(data) != tc.want {
				t.Fatalf("readBulk = %q, want %q", data, tc.want)
			}
		})
	}
}

// TestWrongBulkTrailerIsIssueAndNotPooled discards a mis-framed bulk so the next Get is a new Accept’s own value.
func TestWrongBulkTrailerIsIssueAndNotPooled(t *testing.T) {
	addr := startRawReplyRedis(t, []rawReply{
		{payload: []byte("$5\r\nhello+OK\r\n"), closeAfter: false},
		{payload: []byte("$5\r\nworld\r\n"), closeAfter: true},
	})
	redis := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})

	_, err := redis.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("first Get = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after first Get = %d, want 0", got)
	}

	got, err := redis.Get(context.Background(), "k")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("second Get = %q, want world", got)
	}
}

func TestHeldIntegerSliceSurvivesLaterRead(t *testing.T) {
	addr := startSequentialRedis(t, []string{":111\r\n", ":222\r\n"})
	redis := New(Config{Host: addr})
	first, err := redis.Eval(context.Background(), "return 111", ScriptSHA1Hex("return 111"), nil, nil)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Eval: %v %q", err, first)
	}
	held := first[0]
	second, err := redis.Eval(context.Background(), "return 222", ScriptSHA1Hex("return 222"), nil, nil)
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
	first, err := redis.Eval(context.Background(), "return 'FIRST'", ScriptSHA1Hex("return 'FIRST'"), nil, nil)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Eval: %v %q", err, first)
	}
	held := first[0]
	second, err := redis.Eval(context.Background(), "return 'SECOND'", ScriptSHA1Hex("return 'SECOND'"), nil, nil)
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
	slots, err := redis.Eval(context.Background(), "return {7, 'OK'}", ScriptSHA1Hex("return {7, 'OK'}"), nil, nil)
	if err != nil || len(slots) != 2 {
		t.Fatalf("array Eval: %v %q", err, slots)
	}
	heldInteger := slots[0]
	heldStatus := slots[1]
	later, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil)
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
	_, err := redis.Eval(context.Background(), "return 'x'", ScriptSHA1Hex("return 'x'"), nil, nil)
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
	if err != errIssue { //nolint:errorlint // readLine returns errIssue as the exact sentinel; this test locks that identity
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
	if err != errIssue { //nolint:errorlint // readLine returns errIssue as the exact sentinel; this test locks that identity
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
	_, err := redis.Get(context.Background(), "k")
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
	_, err := redis.Eval(context.Background(), "return {}", ScriptSHA1Hex("return {}"), nil, nil)
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

// TestReadReplyOverCapIsIssue proves over-cap $/* headers and MaxInt64 digits are redis:issue? without a payload read.
func TestReadReplyOverCapIsIssue(t *testing.T) {
	maxIntDigits := strconv.FormatInt(math.MaxInt64, 10)
	tests := []struct {
		name string
		wire string
	}{
		{name: "bulk just over", wire: "$" + strconv.Itoa(maxBulkLength+1) + "\r\n"},
		{name: "array just over", wire: "*" + strconv.Itoa(maxArrayCount+1) + "\r\n"},
		{name: "bulk MaxInt64", wire: "$" + maxIntDigits + "\r\n"},
		{name: "array MaxInt64", wire: "*" + maxIntDigits + "\r\n"},
		{name: "array bulk element over", wire: "*1\r\n$" + strconv.Itoa(maxBulkLength+1) + "\r\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, clean, err := readReply(bufio.NewReader(strings.NewReader(test.wire)))
			if err != errIssue || clean || values != nil { //nolint:errorlint // readReply returns errIssue as the exact sentinel; this test locks that identity
				t.Fatalf("%s: values=%q clean=%v err=%v, want errIssue dirty", test.name, values, clean, err)
			}
		})
	}
}

// TestReadReply256MiBHeaderDoesNotAllocatePayload fails if make still ran for $268435456.
func TestReadReply256MiBHeaderDoesNotAllocatePayload(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	values, clean, err := readReply(bufio.NewReader(strings.NewReader("$268435456\r\n")))
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if err != errIssue || clean || values != nil { //nolint:errorlint // readReply returns errIssue as the exact sentinel; this test locks that identity
		t.Fatalf("256MiB header: values=%q clean=%v err=%v, want errIssue dirty", values, clean, err)
	}
	grew := after.TotalAlloc - before.TotalAlloc
	if grew >= 268435456 {
		t.Fatalf("TotalAlloc grew by %d, payload make still ran", grew)
	}
}

// TestGetOverCapBulkIsIssue proves Get maps an over-cap $ header to redis:issue? and does not pool.
func TestGetOverCapBulkIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "$"+strconv.Itoa(maxBulkLength+1)+"\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("over-cap Get = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after over-cap Get = %d, want 0", got)
	}
}

// TestMGetOverCapBulkElementIsIssue proves an array $ element over the bulk cap is redis:issue? and not pooled.
func TestMGetOverCapBulkElementIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "*1\r\n$"+strconv.Itoa(maxBulkLength+1)+"\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.MGet(context.Background(), []string{"k"})
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("over-cap MGet = %v, want %s", err, RedisIssue)
	}
	if got := pooledIdle(redis); got != 0 {
		t.Fatalf("idle after over-cap MGet = %d, want 0", got)
	}
}
