package simpleredis

import (
	"bufio"
	"context"
	"log/slog"
	"testing"
	"time"
)

// panicOnWrite panics from inside writeCommand's Flush, so the panic happens inside do while
// conn.netConn is still a healthy socket that release can close.
type panicOnWrite struct{}

func (panicOnWrite) Write(_ []byte) (int, error) { panic("simulated panic inside do") }

// TestPanicInDoReturnsTurnAndClosesSocket proves the deferred release in runOnConn returns the
// in-use turn AND destroys the socket when do panics, so neither the turn nor the fd leaks.
// On DestBranch the same panic lost the turn permanently (PR #29 left release non-deferred).
func TestPanicInDoReturnsTurnAndClosesSocket(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	const poolSize = 2
	sr := New(Config{Host: addr, PoolSize: poolSize, PoolTimeout: 50 * time.Millisecond, MaxRetries: -1, Logger: recLogger(h)})
	t.Cleanup(sr.Close)

	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	for i := 0; i < poolSize; i++ {
		conn, err, _ := sr.borrow(context.Background())
		if err != nil {
			t.Fatalf("borrow %d: %v", i, err)
		}
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			conn.writer = bufio.NewWriterSize(panicOnWrite{}, 16)
			_, _ = sr.runOnConn(context.Background(), conn, [][]byte{[]byte("GET"), []byte("hit")})
		}()
		if recovered == nil {
			t.Fatalf("after panic %d: panic did not reach the caller", i)
		}

		if got := len(sr.inUseTurns); got != cap(sr.inUseTurns) {
			t.Fatalf("after panic %d: in-use turns = %d/%d, want full (the deferred release must return the turn)",
				i, got, cap(sr.inUseTurns))
		}
	}

	sr.idleConnsMu.Lock()
	idle := len(sr.idleConns)
	sr.idleConnsMu.Unlock()
	if idle != 0 {
		t.Fatalf("idle sockets after panics = %d, want 0 (a panicked socket has unknown protocol position)", idle)
	}
	if of := sr.OverFrees(); of != 0 {
		t.Fatalf("OverFrees = %d, want 0", of)
	}
	requireMsg(t, h, MsgPanic, slog.LevelError)

	value, err := sr.Get(context.Background(), "hit")
	if err != nil {
		t.Fatalf("Get after %d recovered panics = %v, want success", poolSize, err)
	}
	if string(value) != "t" {
		t.Fatalf("Get = %q, want %q", value, "t")
	}
	t.Logf("turns=%d/%d idle=%d accepts=%d OverFrees=%d",
		len(sr.inUseTurns), cap(sr.inUseTurns), idle, fake.connections(), sr.OverFrees())
}
