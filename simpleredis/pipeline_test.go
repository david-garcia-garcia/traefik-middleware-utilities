package simpleredis

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// countingConn counts Write calls on a client socket so tests can prove one Flush.
type countingConn struct {
	net.Conn
	mu     sync.Mutex
	writes int
}

// Write records one syscall Write then forwards to the inner Conn.
func (c *countingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.writes++
	c.mu.Unlock()
	return c.Conn.Write(p)
}

// writeCount returns how many Write calls the client has made.
func (c *countingConn) writeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes
}

// seedIdle puts netConn on idleConns so the next command reuses it without dialing.
func seedIdle(client *SimpleRedis, netConn net.Conn) {
	client.idleConnsMu.Lock()
	client.idleConns = append(client.idleConns, &pooledConn{
		netConn:  netConn,
		reader:   bufio.NewReader(netConn),
		writer:   bufio.NewWriter(netConn),
		lastUsed: time.Now(),
	})
	client.idleConnsMu.Unlock()
}

// dialCounting seeds idleConns with a counting wrap of a TCP session to addr.
func dialCounting(t *testing.T, client *SimpleRedis, addr string) *countingConn {
	t.Helper()
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	counting := &countingConn{Conn: raw}
	seedIdle(client, counting)
	return counting
}

func TestPipelineOneWriteOrderedReplies(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"k": "v"})
	redis := New(Config{Host: addr})
	counting := dialCounting(t, redis, addr)
	commands := make([][][]byte, 8)
	for i := range commands {
		commands[i] = [][]byte{[]byte("GET"), []byte("k")}
	}
	slots, err := redis.ExecPipeline(commands)
	if err != nil {
		t.Fatalf("ExecPipeline: %v", err)
	}
	if counting.writeCount() != 1 {
		t.Fatalf("Writes = %d, want 1", counting.writeCount())
	}
	if len(slots) != 8 {
		t.Fatalf("slots = %d, want 8", len(slots))
	}
	for i, slot := range slots {
		if slot.Err != nil {
			t.Fatalf("slot %d: %v", i, slot.Err)
		}
		if len(slot.Values) != 1 || string(slot.Values[0]) != "v" {
			t.Fatalf("slot %d = %q, want v", i, slot.Values)
		}
	}
}

func TestPipelineEmptyDoesNotDial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})
	for _, commands := range [][][][]byte{nil, {}} {
		slots, err := redis.ExecPipeline(commands)
		if slots != nil || err != nil {
			t.Fatalf("ExecPipeline(%v) = %v, %v, want nil, nil", commands, slots, err)
		}
	}
	if fake.connections() != 0 {
		t.Fatalf("opened %d connections, want 0", fake.connections())
	}
}

func TestPipelineOverCapDoesNotSend(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})
	commands := make([][][]byte, maxPipelineCommands+1)
	for i := range commands {
		commands[i] = [][]byte{[]byte("PING")}
	}
	slots, err := redis.ExecPipeline(commands)
	if slots != nil {
		t.Fatalf("slots = %v, want nil", slots)
	}
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("err = %v, want %s", err, RedisIssue)
	}
	if fake.connections() != 0 {
		t.Fatalf("opened %d connections, want 0", fake.connections())
	}
}

func TestPipelineElementErrKeepsConn(t *testing.T) {
	store := map[string]string{}
	for i := 0; i < 10; i++ {
		store[fmt.Sprintf("k%d", i)] = "1"
	}
	store["k3"] = "abc"
	fake, addr := startFakeRedis(t, store)
	redis := New(Config{Host: addr})
	_ = dialCounting(t, redis, addr)
	commands := make([][][]byte, 10)
	for i := range commands {
		commands[i] = [][]byte{[]byte("INCR"), []byte(fmt.Sprintf("k%d", i))}
	}
	slots, err := redis.ExecPipeline(commands)
	if err != nil {
		t.Fatalf("batch err: %v", err)
	}
	if len(slots) != 10 {
		t.Fatalf("len = %d, want 10", len(slots))
	}
	if slots[3].Err == nil {
		t.Fatal("slot 3: want -ERR")
	}
	if slots[3].Err.Error() == RedisIssue {
		t.Fatalf("slot 3 = %v, want Redis error text", slots[3].Err)
	}
	for i, slot := range slots {
		if i == 3 {
			continue
		}
		if slot.Err != nil {
			t.Fatalf("slot %d: %v", i, slot.Err)
		}
		if len(slot.Values) != 1 || string(slot.Values[0]) != "2" {
			t.Fatalf("slot %d = %q, want 2", i, slot.Values)
		}
	}
	if _, err := redis.Get("k0"); err != nil {
		t.Fatalf("later Get: %v", err)
	}
	if fake.connections() != 1 {
		t.Fatalf("opened %d connections, want 1", fake.connections())
	}
}

func TestPipelineTruncationIsNotRetried(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var accepts int
	var mu sync.Mutex
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			accepts++
			mu.Unlock()
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for i := 0; i < 3; i++ {
					if _, err := readCommand(reader); err != nil {
						return
					}
				}
				_, _ = io.WriteString(conn, ":1\r\n")
			}(conn)
		}
	}()

	redis := New(Config{Host: listener.Addr().String()})
	commands := [][][]byte{
		{[]byte("INCR"), []byte("a")},
		{[]byte("INCR"), []byte("b")},
		{[]byte("INCR"), []byte("c")},
	}
	_, err = redis.ExecPipeline(commands)
	if err == nil {
		t.Fatal("truncated pipeline: want error")
	}
	mu.Lock()
	got := accepts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("opened %d connections, want 1 (must not retry after Flush)", got)
	}
	redis.idleConnsMu.Lock()
	idle := len(redis.idleConns)
	redis.idleConnsMu.Unlock()
	if idle != 0 {
		t.Fatalf("idle = %d, want 0 (dirty conn must be closed, not pooled)", idle)
	}
}

func TestPipelineTimeoutOnReusedConnIsNotRetried(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var accepts int
	var mu sync.Mutex
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			accepts++
			mu.Unlock()
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				if _, err := readCommand(reader); err != nil {
					return
				}
				_, _ = io.WriteString(conn, "$1\r\nt\r\n")
				time.Sleep(600 * time.Millisecond)
			}(conn)
		}
	}()

	redis := New(Config{Host: listener.Addr().String(), IOTimeout: 200 * time.Millisecond})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	_, err = redis.ExecPipeline([][][]byte{
		{[]byte("GET"), []byte("hit")},
		{[]byte("GET"), []byte("hit")},
	})
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("ExecPipeline = %v, want %s", err, RedisTimeout)
	}
	mu.Lock()
	got := accepts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("opened %d connections, want 1 (timeout must not retry)", got)
	}
}

func TestPipelineDeadIdleRetriedBeforeFlush(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"k": "v"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("k"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	redis.idleConnsMu.Lock()
	for _, conn := range redis.idleConns {
		conn.close()
	}
	redis.idleConnsMu.Unlock()

	slots, err := redis.ExecPipeline([][][]byte{{[]byte("GET"), []byte("k")}})
	if err != nil {
		t.Fatalf("ExecPipeline: %v", err)
	}
	if len(slots) != 1 || slots[0].Err != nil || string(slots[0].Values[0]) != "v" {
		t.Fatalf("slots = %+v", slots)
	}
	if fake.connections() != 2 {
		t.Fatalf("opened %d connections, want 2", fake.connections())
	}
}
