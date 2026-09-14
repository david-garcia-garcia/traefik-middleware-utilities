package simpleredis

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// startStallRedis accepts TCP and never replies, so AUTH/SELECT/GET burn remaining I/O time.
func startStallRedis(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(io.Discard, conn)
			}(conn)
		}
	}()
	return listener.Addr().String()
}

func TestBlackHoleGetReturnsWithinOverallDeadline(t *testing.T) {
	client := newTestRedis(t, Config{Host: "203.0.113.1:6379"})
	budget := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + client.IOTimeout())
	start := time.Now()
	_, err := client.Get(context.Background(), "k")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Get against black hole succeeded")
	}
	if err.Error() != RedisUnreachable && err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s or %s", err, RedisUnreachable, RedisTimeout)
	}
	slack := 50 * time.Millisecond
	if elapsed > budget+slack {
		t.Fatalf("Get elapsed %v, want <= %v", elapsed, budget+slack)
	}
}

func TestHandshakeStallIsBoundedByOverallDeadline(t *testing.T) {
	addr := startStallRedis(t)
	client := newTestRedis(t, Config{Host: addr, Pass: "p", Database: "1"})
	budget := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + client.IOTimeout())
	freshSteps := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + 2*client.IOTimeout())
	start := time.Now()
	_, err := client.Get(context.Background(), "k")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Get against stall succeeded")
	}
	if elapsed > budget+50*time.Millisecond {
		t.Fatalf("Get elapsed %v, want <= overall %v", elapsed, budget+50*time.Millisecond)
	}
	if elapsed >= freshSteps {
		t.Fatalf("Get elapsed %v, want < DialTimeout+2×IOTimeout per attempt %v", elapsed, freshSteps)
	}
}

func TestGetCancelFreesTurnAndDoesNotPool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })

	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, IOTimeout: 5 * time.Second, MaxRetries: -1})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Get(ctx, "hit")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		fake.mu.Lock()
		held := fake.heldGets
		fake.mu.Unlock()
		if held > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Get never held")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get = %v, want context.Canceled", err)
	}
	client.idleConnsMu.Lock()
	idleAfter := len(client.idleConns)
	client.idleConnsMu.Unlock()
	if idleAfter != 0 {
		t.Fatalf("idle after cancel = %d, want 0", idleAfter)
	}

	fake.mu.Lock()
	fake.holdCh = nil
	fake.mu.Unlock()
	if _, err := client.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
}

func TestGetCancelWhileWaitingForTurn(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })

	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, PoolTimeout: time.Second, IOTimeout: 5 * time.Second, MaxRetries: -1})
	go func() {
		_, _ = client.Get(context.Background(), "hit")
	}()
	deadline := time.Now().Add(time.Second)
	for {
		fake.mu.Lock()
		held := fake.heldGets
		fake.mu.Unlock()
		if held > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder never held")
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error, 1)
	go func() {
		_, err := client.Get(ctx, "hit")
		waitErr <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	err := <-waitErr
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter Get = %v, want context.Canceled", err)
	}
	if fake.connections() != 1 {
		t.Fatalf("connections = %d, want 1 (waiter must not dial)", fake.connections())
	}
	if n := len(client.inUseTurns); n != 0 {
		t.Fatalf("inUseTurns = %d, want 0 (holder still holds the only turn)", n)
	}
}

func TestGetAlreadyCancelledDoesNotSend(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	client := newTestRedis(t, Config{Host: addr})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Get(ctx, "hit")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get = %v, want context.Canceled", err)
	}
	if fake.connections() != 0 {
		t.Fatalf("connections = %d, want 0", fake.connections())
	}
}

func TestZeroConfigMaxRetriesIsOneExtra(t *testing.T) {
	client := newTestRedis(t, Config{Host: "127.0.0.1:1"})
	if client.MaxRetries() != 1 {
		t.Fatalf("MaxRetries() = %d, want 1", client.MaxRetries())
	}
	if client.DialTimeout() != 200*time.Millisecond {
		t.Fatalf("DialTimeout() = %v, want 200ms", client.DialTimeout())
	}
	if client.IOTimeout() != 250*time.Millisecond {
		t.Fatalf("IOTimeout() = %v, want 250ms", client.IOTimeout())
	}
}

func TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout(t *testing.T) {
	addr := startStallRedis(t)
	client := newTestRedis(t, Config{Host: addr, MaxRetries: -1, DialTimeout: time.Second, IOTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := client.Get(ctx, "k")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get = %v, want context.DeadlineExceeded (caller deadline, not %s)", err, RedisTimeout)
	}
}

func TestGetCallerDeadlineWhileWaitingForTurn(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })

	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, PoolTimeout: time.Second, IOTimeout: 5 * time.Second, MaxRetries: -1})
	go func() {
		_, _ = client.Get(context.Background(), "hit")
	}()
	deadline := time.Now().Add(time.Second)
	for {
		fake.mu.Lock()
		held := fake.heldGets
		fake.mu.Unlock()
		if held > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder never held")
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := client.Get(ctx, "hit")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter Get = %v, want context.DeadlineExceeded", err)
	}
	if fake.connections() != 1 {
		t.Fatalf("connections = %d, want 1 (waiter must not dial)", fake.connections())
	}
}
