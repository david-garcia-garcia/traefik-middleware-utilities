package simpleredis

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// peerDropAllFake is a compliant RESP peer whose accepted sockets can all be dropped at once.
// Sequential Gets after that drop are how dest spends MaxRetries+1 corpses per request.
type peerDropAllFake struct {
	mu       sync.Mutex
	store    map[string]string
	conns    []net.Conn
	heldGets int
	hold     chan struct{}
}

// startPeerDropAllFake listens on loopback and serves store over real TCP.
func startPeerDropAllFake(t *testing.T, store map[string]string) (fake *peerDropAllFake, listenAddr string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fake = &peerDropAllFake{store: store}
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			fake.mu.Lock()
			fake.conns = append(fake.conns, conn)
			fake.mu.Unlock()
			go fake.serve(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// serve answers GET (and other commands as +OK) on one accepted socket, and can hold GETs to warm idle.
func (f *peerDropAllFake) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		reply := statusOKReply
		if args[0] == "GET" {
			reply = bulk(f.store, args[1])
		}
		hold := f.hold
		if hold != nil && args[0] == "GET" {
			f.heldGets++
			f.mu.Unlock()
			<-hold
			f.mu.Lock()
			f.heldGets--
		}
		f.mu.Unlock()
		if _, err := io.WriteString(conn, reply); err != nil {
			return
		}
	}
}

// dropEveryAcceptedSocket closes every server fd the way a restart or CLIENT KILL of the pool does.
func (f *peerDropAllFake) dropEveryAcceptedSocket() {
	f.mu.Lock()
	conns := f.conns
	f.conns = nil
	f.mu.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
}

// warmPeerDropAllIdle parks exactly n sockets by holding n Gets at the server at once.
// Sequential Gets would reuse one socket and would not fill the idle vintage.
func warmPeerDropAllIdle(t *testing.T, fake *peerDropAllFake, sr *SimpleRedis, n int) {
	t.Helper()
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.hold = hold
	fake.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := sr.Get(context.Background(), "hit"); err != nil {
				t.Errorf("warm Get: %v", err)
			}
		}()
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		fake.mu.Lock()
		held := fake.heldGets
		fake.mu.Unlock()
		if held >= n {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d Gets reached the server", held, n)
		}
		time.Sleep(time.Millisecond)
	}

	fake.mu.Lock()
	fake.hold = nil
	fake.mu.Unlock()
	close(hold)
	wg.Wait()

	deadline = time.Now().Add(2 * time.Second)
	for pooledIdle(sr) < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := pooledIdle(sr); got != n {
		t.Fatalf("warmed idle = %d, want %d", got, n)
	}
}

// TestPeerDropAllSequentialGetsSucceedAfterPeerDrop proves dest's quiet-path burst is gone:
// after every idle socket is dropped, sequential Gets still succeed against a peer that stayed up.
func TestPeerDropAllSequentialGetsSucceedAfterPeerDrop(t *testing.T) {
	const poolSize = 8
	fake, addr := startPeerDropAllFake(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, PoolSize: poolSize, MaxIdleConns: poolSize,
		MinRetryBackoff: -1, MaxRetryBackoff: -1})
	t.Cleanup(sr.Close)

	warmPeerDropAllIdle(t, fake, sr, poolSize)
	fake.dropEveryAcceptedSocket()
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < poolSize; i++ {
		got, err := sr.Get(context.Background(), "hit")
		if err != nil {
			t.Fatalf("sequential Get %d: %v", i, err)
		}
		if string(got) != "t" {
			t.Fatalf("sequential Get %d = %q, want t", i, got)
		}
	}
	assertTurnsFullAndNoOverFrees(t, sr)
}
