package tokenbucket

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestTraefik14328_MemoryConcurrentFirstRequests proves 16 simultaneous first Allows on a new in-process
// source admit at most burst (Traefik #14328: ttlmap Get/Set race does not happen here because Memory holds
// one mutex across lookup, create, and consume).
func TestTraefik14328_MemoryConcurrentFirstRequests(t *testing.T) {
	const (
		rounds     = 20
		goroutines = 16
		burst      = 5
		rate       = 1.0 / 60
		maxDelay   = 500 * time.Millisecond
		ttl        = 120 * time.Second
		sourceKey  = "192.0.2.1"
	)
	maxAdmits := 0
	for round := 0; round < rounds; round++ {
		limiter, err := NewMemory(rate, burst, maxDelay, ttl)
		if err != nil {
			t.Fatal(err)
		}
		got := concurrentFirstAdmits14328(t, limiter.Allow, sourceKey, goroutines, burst)
		if got > maxAdmits {
			maxAdmits = got
		}
	}
	t.Logf("max admits across %d rounds: %d", rounds, maxAdmits)
}

// concurrentFirstAdmits14328 starts goroutines, waits until they all sit on start, then lets them Allow at once.
// It counts Status.Allowed and fails the test on a non-nil Allow error or when admits exceed burst.
func concurrentFirstAdmits14328(t *testing.T, allow func(context.Context, string) (Status, error), key string, goroutines, burst int) int {
	t.Helper()
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	var mu sync.Mutex
	var firstErr error
	admits := 0
	// Park every goroutine on start so the first Allow cannot run before the last is ready.
	ready.Add(goroutines)
	done.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			status, err := allow(context.Background(), key)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if status.Allowed {
				admits++
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	// A non-nil Allow error or admits past burst is the Traefik #14328 failure mode.
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if admits > burst {
		t.Fatalf("concurrent first requests admitted %d, want at most burst %d", admits, burst)
	}
	return admits
}
