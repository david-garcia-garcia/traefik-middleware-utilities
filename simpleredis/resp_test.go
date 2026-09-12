package simpleredis

import (
	"net"
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
