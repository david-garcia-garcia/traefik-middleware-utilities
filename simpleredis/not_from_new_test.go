package simpleredis

import (
	"testing"
	"time"
)

// TestZeroValueGetDoesNotRetry is a Get on a client that did not come from New: redis:unreachable without the retry ladder.
func TestZeroValueGetDoesNotRetry(t *testing.T) {
	sr := &SimpleRedis{}
	if cap(sr.inUseTurns) != 0 {
		t.Fatalf("cap(inUseTurns) = %d, want 0 so New remains the only semaphore constructor", cap(sr.inUseTurns))
	}
	start := time.Now()
	_, err := sr.Get("k")
	elapsed := time.Since(start)
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if elapsed >= 8*time.Millisecond {
		t.Fatalf("Get elapsed %v, want < 8ms so the retry ladder did not run", elapsed)
	}
}

// TestZeroValueExportedMethodsDoNotPanic smokes every exported command on a client that did not come from New, including Close twice.
func TestZeroValueExportedMethodsDoNotPanic(t *testing.T) {
	sr := &SimpleRedis{}
	calls := []struct {
		name string
		run  func() error
	}{
		{name: "Get", run: func() error { _, err := sr.Get("k"); return err }},
		{name: "MGet", run: func() error { _, err := sr.MGet([]string{"k"}); return err }},
		{name: "Set", run: func() error { return sr.Set("k", []byte("v"), 1) }},
		{name: "Del", run: func() error { return sr.Del("k") }},
		{name: "Incr", run: func() error { _, err := sr.Incr("k"); return err }},
		{name: "IncrBy", run: func() error { _, err := sr.IncrBy("k", 1); return err }},
		{name: "Expire", run: func() error { return sr.Expire("k", 1) }},
		{name: "ExpireAt", run: func() error { return sr.ExpireAt("k", 1) }},
		{name: "Eval", run: func() error { _, err := sr.Eval("return 1", nil, nil); return err }},
		{name: "MSetEX", run: func() error { return sr.MSetEX([]string{"k"}, [][]byte{[]byte("v")}, 1) }},
		{name: "MSetEXAt", run: func() error { return sr.MSetEXAt([]string{"k"}, [][]byte{[]byte("v")}, 1) }},
	}
	for _, call := range calls {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("%s panicked: %v", call.name, recovered)
				}
			}()
			err := call.run()
			if err == nil || err.Error() != RedisUnreachable {
				t.Fatalf("%s = %v, want %s", call.name, err, RedisUnreachable)
			}
		}()
	}
	sr.Close()
	sr.Close()
}
