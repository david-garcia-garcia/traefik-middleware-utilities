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
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()
	return listener.Addr().String()
}

func TestBlackHoleGetReturnsWithinOverallDeadline(t *testing.T) {
	client := New(Config{Host: "203.0.113.1:6379"})
	budget := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + client.IOTimeout())
	start := time.Now()
	_, err := client.Get("k")
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
	client := New(Config{Host: addr, Pass: "p", Database: "1"})
	budget := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + client.IOTimeout())
	freshSteps := time.Duration(client.MaxRetries()+1) * (client.DialTimeout() + 2*client.IOTimeout())
	start := time.Now()
	_, err := client.Get("k")
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

func TestGetContextCancelFreesTurnAndDoesNotPool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })

	client := New(Config{Host: addr, PoolSize: 1, IOTimeout: 5 * time.Second, MaxRetries: -1})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.GetContext(ctx, "hit")
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
			t.Fatal("GetContext never held")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetContext = %v, want context.Canceled", err)
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
	if _, err := client.Get("hit"); err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
}

func TestGetContextAlreadyCancelledDoesNotSend(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	client := New(Config{Host: addr})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetContext(ctx, "hit")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetContext = %v, want context.Canceled", err)
	}
	if fake.connections() != 0 {
		t.Fatalf("connections = %d, want 0", fake.connections())
	}
}

func TestZeroConfigMaxRetriesIsOneExtra(t *testing.T) {
	client := New(Config{Host: "127.0.0.1:1"})
	if client.MaxRetries() != 1 {
		t.Fatalf("MaxRetries() = %d, want 1", client.MaxRetries())
	}
	if client.DialTimeout() != 200*time.Millisecond {
		t.Fatalf("DialTimeout() = %v, want 200ms", client.DialTimeout())
	}
	if client.IOTimeout() != 100*time.Millisecond {
		t.Fatalf("IOTimeout() = %v, want 100ms", client.IOTimeout())
	}
}
