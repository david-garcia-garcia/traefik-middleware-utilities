package simpleredis

import (
	"context"
	"net"
	"testing"
	"time"
)

// panicOnWriteConn panics from inside writeCommand's Write, so the panic happens inside do while
// conn.netConn is still a healthy socket that release can close.
type panicOnWriteConn struct {
	net.Conn
}

func (panicOnWriteConn) Write(_ []byte) (int, error) { panic("simulated panic inside do") }

// TestPanicInDoReturnsTurnAndClosesSocket proves the deferred release in runOnConn returns the
// in-use turn AND destroys the socket when do panics, so neither the turn nor the fd leaks.
// On DestBranch the same panic lost the turn permanently (PR #29 left release non-deferred).
func TestPanicInDoReturnsTurnAndClosesSocket(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	const poolSize = 2
	sr := newTestRedis(t, Config{Host: addr, PoolSize: poolSize, MaxIdleConns: poolSize, PoolTimeout: 50 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(sr.Close)

	// Warm one socket so the panic hits a pooled connection, not a fresh dial.
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	for i := 0; i < poolSize; i++ {
		conn, err, _, _ := sr.borrowSocket(context.Background(), false)
		if err != nil {
			t.Fatalf("borrow %d: %v", i, err)
		}
		// Traefik shape: the middleware panics and Traefik recovers the request.
		func() {
			defer func() { _ = recover() }()
			conn.netConn = panicOnWriteConn{conn.netConn}
			_, _ = sr.runOnConn(context.Background(), conn, [][]byte{[]byte("GET"), []byte("hit")})
		}()

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

	// The client must still work: this is what a bricked pool could not do.
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
