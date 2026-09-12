package simpleredis

import (
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

func TestEvalNestedArrayIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "*1\r\n*0\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Eval("return {{}}", nil, nil)
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Eval nested = %v, want %s", err, RedisIssue)
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

func TestLongStatusLineDecodes(t *testing.T) {
	payload := strings.Repeat("A", 5000)
	addr := startStaticRedis(t, "+"+payload+"\r\n")
	redis := New(Config{Host: addr})
	values, err := redis.Eval("return 'x'", nil, nil)
	if err != nil {
		t.Fatalf("long status: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("long status values=%q", values)
	}
	if string(values[0]) != payload {
		t.Fatalf("long status len=%d, want %d", len(values[0]), len(payload))
	}
}

func TestGarbageBulkLengthIsIssue(t *testing.T) {
	addr := startStaticRedis(t, "$abc\r\n")
	redis := New(Config{Host: addr})
	_, err := redis.Get("k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("garbage bulk = %v, want %s", err, RedisIssue)
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
