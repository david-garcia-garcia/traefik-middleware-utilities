package tokenbucket

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// runNonce14173 keeps keys from one run apart from keys a previous run left on the same server.
var runNonce14173 = strconv.FormatInt(time.Now().UnixNano(), 36)

// countingProxy14173 is a loopback TCP proxy that can hold client writes so a request already read still reaches the
// server after the client leaves.
type countingProxy14173 struct {
	listener   net.Listener
	holdWrites atomic.Int64 // nanoseconds each client-to-server chunk waits before it is forwarded
}

// liveKey14173 is a per-run, per-test Redis key.
func liveKey14173(t *testing.T, suffix string) string {
	t.Helper()
	return "tb-14173:" + runNonce14173 + ":" + t.Name() + ":" + suffix
}

// evalKey14173 runs a test script on KEYS[1]; scripts touch only that key so Dragonfly accepts them.
func evalKey14173(t *testing.T, client *simpleredis.SimpleRedis, script, key string, args ...string) [][]byte {
	t.Helper()
	values, err := client.Eval(context.Background(), script, simpleredis.ScriptSHA1Hex(script), []string{key}, args)
	if err != nil {
		t.Fatalf("eval on %s: %v", key, err)
	}
	return values
}

// startCountingProxy14173 forwards every accepted connection to upstream until the test ends.
func startCountingProxy14173(t *testing.T, upstream string) *countingProxy14173 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &countingProxy14173{listener: listener}
	go func() {
		for {
			down, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			up, dialErr := net.Dial("tcp", upstream)
			if dialErr != nil {
				_ = down.Close()
				continue
			}
			go func() {
				var pipes sync.WaitGroup
				pipes.Add(2)
				go func() { defer pipes.Done(); p.forwardHeld14173(up, down); _ = up.Close() }()
				go func() { defer pipes.Done(); _, _ = io.Copy(down, up); _ = down.Close() }()
				pipes.Wait()
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return p
}

// forwardHeld14173 copies src to dst, waiting holdWrites before each chunk. A chunk read before src closes is still
// delivered, the way a request already on the wire reaches the server after the client leaves.
func (p *countingProxy14173) forwardHeld14173(dst io.Writer, src io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if hold := time.Duration(p.holdWrites.Load()); hold > 0 {
				time.Sleep(hold)
			}
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}

// TestTraefik14173_RedisCanceledInFlight proves Allow returns context.Canceled with Allowed false while EVAL is held on
// the wire, so the middleware can answer 499 instead of waiting for the server. Whether the held script then charged
// the source is logged, not asserted.
func TestTraefik14173_RedisCanceledInFlight(t *testing.T) {
	runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", runCanceledInFlight14173)
}

// runCanceledInFlight14173 cancels Allow while a holding proxy delays the EVALSHA write to one live engine.
func runCanceledInFlight14173(t *testing.T, addr string) {
	t.Helper()
	const (
		burst = int64(3)
		hold  = 1500 * time.Millisecond
	)
	key := liveKey14173(t, "client-1")
	direct := waitLiveClient(t, addr)
	proxy := startCountingProxy14173(t, addr)
	client := waitLiveClient(t, proxy.listener.Addr().String())
	limiter, err := NewRedis(client, 1.0/60, burst, 0, 120*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// Load the allow script and warm the pooled socket so the canceled call is a single EVALSHA write.
	if _, err := limiter.Allow(context.Background(), liveKey14173(t, "warmup")); err != nil {
		t.Fatal(err)
	}

	proxy.holdWrites.Store(int64(hold))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)
	started := time.Now()
	status, err := limiter.Allow(ctx, key)
	elapsed := time.Since(started)
	t.Logf("canceled in flight: allowed=%v err=%v after %v", status.Allowed, err, elapsed)

	time.Sleep(hold + 300*time.Millisecond) // let the held EVAL reach the server
	state := evalKey14173(t, direct, "return redis.call('hmget', KEYS[1], 'last', 'tokens')", key)
	last, tokens := []byte(nil), []byte(nil)
	if len(state) > 0 {
		last = state[0]
	}
	if len(state) > 1 {
		tokens = state[1]
	}
	t.Logf("bucket once the held EVAL was delivered: last=%q tokens=%q (burst %d; empty = never ran, %d = one token charged)", last, tokens, burst, burst-1)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
	if status.Allowed {
		t.Fatal("canceled request reported allowed")
	}
	if elapsed > hold/2 {
		t.Fatalf("Allow returned after %v, want it to return on cancel (~200 ms), not when the server answered", elapsed)
	}
}
