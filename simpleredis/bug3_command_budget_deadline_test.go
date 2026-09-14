package simpleredis

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// startBug3ChunkedBulkRedis serves one $ payloadLen bulk in chunk-sized writes with
// perChunk sleep between them. silentAfter >= 0 stops after that many payload bytes
// and waits for the client to close. Prefix bug3 so sibling branches' fakes do not collide.
func startBug3ChunkedBulkRedis(t *testing.T, payloadLen, chunk int, fill byte, perChunk time.Duration, silentAfter int) string {
	t.Helper()
	if chunk <= 0 {
		t.Fatal("chunk must be > 0")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go bug3ServeChunkedBulk(conn, payloadLen, chunk, fill, perChunk, silentAfter)
		}
	}()
	return listener.Addr().String()
}

func bug3ServeChunkedBulk(conn net.Conn, payloadLen, chunk int, fill byte, perChunk time.Duration, silentAfter int) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	body := bytes.Repeat([]byte{fill}, chunk)
	for {
		if _, err := readCommand(reader); err != nil {
			return
		}
		if _, err := io.WriteString(conn, fmt.Sprintf("$%d\r\n", payloadLen)); err != nil {
			return
		}
		for sent := 0; sent < payloadLen; sent += chunk {
			if silentAfter >= 0 && sent >= silentAfter {
				_, _ = io.Copy(io.Discard, conn)
				return
			}
			if perChunk > 0 {
				time.Sleep(perChunk)
			}
			n := chunk
			if sent+n > payloadLen {
				n = payloadLen - sent
			}
			if _, err := conn.Write(body[:n]); err != nil {
				return
			}
		}
		if _, err := io.WriteString(conn, "\r\n"); err != nil {
			return
		}
	}
}

func TestBug3StreamingBulkBeyondIOTimeoutReturnsIntact(t *testing.T) {
	const payloadLen = 1 << 20 // 1 MiB; chunk pauses make transfer > IOTimeout
	const fill byte = 0x5A
	addr := startBug3ChunkedBulkRedis(t, payloadLen, 32<<10, fill, 4*time.Millisecond, -1)
	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, IOTimeout: 40 * time.Millisecond,
		MaxRetries: -1, MinRetryBackoff: -1, MaxRetryBackoff: -1})
	t.Cleanup(client.Close)

	got, err := client.Get(context.Background(), "big")
	if err != nil {
		t.Fatalf("Get streaming bulk: %v", err)
	}
	want := bytes.Repeat([]byte{fill}, payloadLen)
	if !bytes.Equal(got, want) {
		t.Fatalf("Get payload len=%d first=%v, want len=%d fill=0x%02x", len(got), prefixBug3(got), payloadLen, fill)
	}
	assertTurnsFullAndNoOverFrees(t, client)
}

// TestBug3SilentMidReplyTimesOutAtCommandBudget pins the price of stamping the socket deadline from
// the command budget: a peer that goes quiet mid-reply is no longer cut off after IOTimeout, it ends
// the command when the budget runs out. That is the deliberate trade for not killing a large reply
// that is still arriving, so the lower bound is asserted too — losing it would mean the socket
// deadline drifted back to a per-operation cap.
func TestBug3SilentMidReplyTimesOutAtCommandBudget(t *testing.T) {
	addr := startBug3ChunkedBulkRedis(t, 4096, 16, 0x11, 0, 16)
	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, IOTimeout: 40 * time.Millisecond,
		MaxRetries: -1, MinRetryBackoff: -1, MaxRetryBackoff: -1, DialTimeout: 200 * time.Millisecond})
	t.Cleanup(client.Close)

	retries, _, _ := retryLimits(client.MaxRetries(), client.minRetryBackoff, client.maxRetryBackoff)
	budget := time.Duration(retries+1) * (client.DialTimeout() + client.IOTimeout())
	start := time.Now()
	_, err := client.Get(context.Background(), "partial")
	elapsed := time.Since(start)
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get silent mid-reply = %v, want %s", err, RedisTimeout)
	}
	if elapsed > budget+100*time.Millisecond {
		t.Fatalf("Get silent mid-reply elapsed %v, want <= budget %v plus slack", elapsed, budget)
	}
	if elapsed < client.IOTimeout() {
		t.Fatalf("Get silent mid-reply elapsed %v, want at least IOTimeout %v", elapsed, client.IOTimeout())
	}
	assertTurnsFullAndNoOverFrees(t, client)
}

func TestBug3DripPeerCannotPinTurnPastOverallBudget(t *testing.T) {
	// 1-byte chunks every 20ms keep arriving forever, so only the command budget can end this.
	addr := startBug3ChunkedBulkRedis(t, 1<<20, 1, 0x22, 20*time.Millisecond, -1)
	client := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, IOTimeout: 50 * time.Millisecond, DialTimeout: 50 * time.Millisecond,
		MaxRetries: -1, MinRetryBackoff: -1, MaxRetryBackoff: -1, PoolTimeout: 80 * time.Millisecond})
	t.Cleanup(client.Close)

	retries, _, _ := retryLimits(client.MaxRetries(), client.minRetryBackoff, client.maxRetryBackoff)
	budget := time.Duration(retries+1) * (client.DialTimeout() + client.IOTimeout())
	start := time.Now()
	_, err := client.Get(context.Background(), "drip")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Get drip peer succeeded, want overall-budget timeout")
	}
	if elapsed > budget+50*time.Millisecond {
		t.Fatalf("Get drip elapsed %v, want <= overall %v + slack", elapsed, budget)
	}
	assertTurnsFullAndNoOverFrees(t, client)

	start = time.Now()
	_, err = client.Get(context.Background(), "drip")
	elapsed = time.Since(start)
	if err == nil {
		t.Fatal("second Get drip succeeded, want timeout (turn must have been free)")
	}
	if elapsed > budget+50*time.Millisecond {
		t.Fatalf("second Get elapsed %v, want <= overall %v + slack (leaked turn would wait PoolTimeout)", elapsed, budget)
	}
	assertTurnsFullAndNoOverFrees(t, client)
}

func prefixBug3(payload []byte) []byte {
	if len(payload) > 8 {
		return payload[:8]
	}
	return payload
}
