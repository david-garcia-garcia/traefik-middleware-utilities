package simpleredis

import (
	"bufio"
	"context"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	chaosKeyCount     = 64
	chaosPoolSize     = 4
	chaosHonest       = 0
	chaosDelay        = 1
	chaosCloseNoReply = 2
	chaosLoading      = 3
	chaosTruncated    = 4
	chaosCmdAuth      = "AUTH"
	chaosCmdSelect    = "SELECT"
	chaosCmdGet       = "GET"
)

// chaosIntn is the chaos mix. Not a secret.
func chaosIntn(n int) int {
	return rand.Intn(n) //nolint:gosec // G404: test chaos mix
}

// chaosFake is an in-process RESP server that, per command after handshake, randomly
// replies honestly, delays, closes with no reply, replies LOADING, or truncates a bulk.
type chaosFake struct {
	mu     sync.Mutex
	store  map[string]string
	open   int
	accept int
}

// startChaosFake serves k0..k63 as v0..v63 and tracks still-open accepted sockets.
func startChaosFake(t *testing.T) (server *chaosFake, listenAddr string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	store := make(map[string]string, chaosKeyCount)
	for i := 0; i < chaosKeyCount; i++ {
		store["k"+strconv.Itoa(i)] = "v" + strconv.Itoa(i)
	}
	fake := &chaosFake{store: store}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fake.mu.Lock()
			fake.accept++
			fake.open++
			fake.mu.Unlock()
			go fake.serve(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// serve answers AUTH/SELECT honestly, then randomly honest / delay / close / LOADING / truncated bulk.
func (f *chaosFake) serve(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		f.mu.Lock()
		f.open--
		f.mu.Unlock()
	}()
	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		if args[0] == chaosCmdAuth || args[0] == chaosCmdSelect {
			if _, err := io.WriteString(conn, statusOKReply); err != nil {
				return
			}
			continue
		}
		switch chaosIntn(5) {
		case chaosHonest:
			if _, err := io.WriteString(conn, f.honestReply(args)); err != nil {
				return
			}
		case chaosDelay:
			time.Sleep(time.Duration(chaosIntn(4)) * time.Millisecond)
			if _, err := io.WriteString(conn, f.honestReply(args)); err != nil {
				return
			}
		case chaosCloseNoReply:
			return
		case chaosLoading:
			if _, err := io.WriteString(conn, "-LOADING Redis is loading the dataset in memory\r\n"); err != nil {
				return
			}
		case chaosTruncated:
			if _, err := io.WriteString(conn, "$100\r\nshort"); err != nil {
				return
			}
			return
		}
	}
}

// honestReply is GET kN → vN, otherwise +OK.
func (f *chaosFake) honestReply(args []string) string {
	if args[0] == chaosCmdGet && len(args) > 1 {
		return bulk(f.store, args[1])
	}
	return statusOKReply
}

// openSockets is how many accepted sockets are still open.
func (f *chaosFake) openSockets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open
}

// accepts is how many TCP accepts the chaos fake has seen.
func (f *chaosFake) accepts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accept
}

// waitOpenSocketsAtMost waits until settled open sockets are at most want.
func (f *chaosFake) waitOpenSocketsAtMost(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		open := f.openSockets()
		if open <= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("settled open sockets = %d, want <= %d", open, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestChaosPoolInvariants locks the healthy chaos-pool probe: no cross-key Get,
// no leaked turns, no over-frees, settled open sockets at most PoolSize.
//
// A sampler that reads len(idleConns) and then len(inUseTurns) is NOT a valid
// live-socket metric and will report up to 2x PoolSize. Two independent reasons:
// the two reads are not atomic, AND release publishes the socket into idleConns
// BEFORE calling freeInUseTurn, so a releasing socket is legitimately counted
// in both. Server-side PEAK open sockets also over-count, because the server
// goroutine's exit lags the client's Close by a scheduling hop. Only settled
// / at-rest values are sound. Do not assert on peak live sockets — it will flake.
func TestChaosPoolInvariants(t *testing.T) {
	if testing.Short() {
		t.Skip("chaos pool stress")
	}
	workers := 24
	duration := time.Second
	if raceDetectorOn {
		workers = 8
		duration = 250 * time.Millisecond
	}

	fake, addr := startChaosFake(t)
	client := New(Config{Host: addr, PoolSize: chaosPoolSize, MaxRetries: 1, IOTimeout: 50 * time.Millisecond, PoolTimeout: 50 * time.Millisecond})
	t.Cleanup(client.Close)

	var crossTalk atomic.Int64
	stopAt := time.Now().Add(duration)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for time.Now().Before(stopAt) {
				keyIndex := chaosIntn(chaosKeyCount)
				name := "k" + strconv.Itoa(keyIndex)
				want := "v" + strconv.Itoa(keyIndex)
				ctx, cancel := chaosGetContext()
				got, err := client.Get(ctx, name)
				cancel()
				if err == nil && string(got) != want {
					crossTalk.Add(1)
					t.Errorf("Get(%s) = %q, want %s", name, got, want)
				}
			}
		}()
	}
	wg.Wait()

	if got := crossTalk.Load(); got != 0 {
		t.Fatalf("cross-key Gets = %d, want 0", got)
	}
	fake.waitOpenSocketsAtMost(t, chaosPoolSize)
	assertTurnsFullAndNoOverFrees(t, client)
	if fake.accepts() == 0 {
		t.Fatal("chaos fake accepted 0 connections")
	}
}

// chaosGetContext is Background, a short timeout, or cancel soon via AfterFunc.
func chaosGetContext() (context.Context, context.CancelFunc) {
	switch chaosIntn(3) {
	case 0:
		return context.Background(), func() {}
	case 1:
		return context.WithTimeout(context.Background(), 8*time.Millisecond)
	default:
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(time.Duration(chaosIntn(4)+1)*time.Millisecond, cancel)
		return ctx, cancel
	}
}
