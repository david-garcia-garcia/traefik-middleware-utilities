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

func TestAppendRESPGetMatchesDestFraming(t *testing.T) {
	got := appendRESP(nil, [][]byte{[]byte("GET"), []byte("session:9f2c1ab4-user-token")})
	want := "*2\r\n$3\r\nGET\r\n$27\r\nsession:9f2c1ab4-user-token\r\n"
	if string(got) != want {
		t.Fatalf("appendRESP GET = %q, want %q", got, want)
	}
}
