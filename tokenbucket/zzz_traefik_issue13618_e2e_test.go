package tokenbucket

import (
	"context"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// testCountingProxy13618 is a loopback TCP proxy that counts open client connections.
type testCountingProxy13618 struct {
	listener net.Listener
	open     atomic.Int64
}

// startTestCountingProxy13618 forwards every accepted connection to upstream until the test ends.
func startTestCountingProxy13618(t *testing.T, upstream string) *testCountingProxy13618 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &testCountingProxy13618{listener: listener}
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
			proxy.open.Add(1)
			go func() {
				defer proxy.open.Add(-1)
				var pipes sync.WaitGroup
				pipes.Add(2)
				go func() { defer pipes.Done(); _, _ = io.Copy(up, down); _ = up.Close() }()
				go func() { defer pipes.Done(); _, _ = io.Copy(down, up); _ = down.Close() }()
				pipes.Wait()
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return proxy
}

// liveKey13618 is a per-test Redis key for this issue's live proof.
func liveKey13618(t *testing.T, suffix string) string {
	t.Helper()
	return "tb-13618:" + t.Name() + ":" + suffix
}

// TestTraefik13618_ReloadWithoutClose: traefik/traefik#13618.
// Traefik builds a Redis client per middleware instance on every reload and never closes it; go-redis
// v9 pins each orphan with a goroutine, so goroutines and memory grow per reload. This simulates
// 50 reloads the same way: a new simpleredis client and limiter, one Allow, then dropped without Close.
// After GC, no proxied socket stays open and leaked goroutines excluding the proxy's own fail only
// when greater than 2.
func TestTraefik13618_ReloadWithoutClose(t *testing.T) {
	runForEachLiveEngine(t, func(t *testing.T, addr string) {
		t.Helper()
		const reloads = 50
		liveClient := waitLiveClient(t, addr)
		proxy := startTestCountingProxy13618(t, addr)
		liveClient.Close()
		key := liveKey13618(t, "client-1")

		// settle runs several GC passes so unreachable clients can release sockets.
		settle := func() {
			for i := 0; i < 5; i++ {
				runtime.GC()
				time.Sleep(50 * time.Millisecond)
			}
		}
		settle()
		before := runtime.NumGoroutine()

		// reload builds one client and limiter, runs Allow, and drops both without Close.
		reload := func(i int) {
			client, err := simpleredis.New(simpleredis.Config{Host: proxy.listener.Addr().String()})
			if err != nil {
				t.Fatal(err)
			}
			limiter, err := NewRedis(client, 100, 50, 0, 120*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			status, err := limiter.Allow(context.Background(), key)
			if err != nil {
				t.Fatalf("reload %d: %v", i, err)
			}
			_ = status
		}
		for i := 0; i < reloads; i++ {
			reload(i)
		}
		peakSockets := proxy.open.Load()

		deadline := time.Now().Add(5 * time.Second)
		for proxy.open.Load() > 0 && time.Now().Before(deadline) {
			settle()
		}
		settle()
		after := runtime.NumGoroutine()
		// Each open proxied socket costs the proxy 3 goroutines; count only the rest.
		leakedGoroutines := after - before - 3*int(proxy.open.Load())
		t.Logf("after %d reloads: sockets open at peak %d, after GC %d; goroutines before %d, after %d (excluding proxy: %+d)",
			reloads, peakSockets, proxy.open.Load(), before, after, leakedGoroutines)

		if open := proxy.open.Load(); open != 0 {
			t.Errorf("%d of %d sockets from dropped clients still open after GC", open, peakSockets)
		}
		if leakedGoroutines > 2 {
			t.Errorf("%d goroutines survive %d dropped clients", leakedGoroutines, reloads)
		}
	})
}
