package simpleredis

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestLifecycleNewUseCloseDoesNotLeak locks 200 New/use/Close cycles: goroutine
// count stays flat and server-side open sockets end at 0.
func TestLifecycleNewUseCloseDoesNotLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("lifecycle New/use/Close stress")
	}
	cycles := 200
	if raceDetectorOn {
		cycles = 40
	}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	runtime.GC()
	goroutinesBefore := runtime.NumGoroutine()

	for i := 0; i < cycles; i++ {
		client := newTestRedis(t, Config{Host: addr, PoolSize: 4, MaxIdleConns: 4, MaxRetries: -1})
		var wg sync.WaitGroup
		wg.Add(4)
		for j := 0; j < 4; j++ {
			go func() {
				defer wg.Done()
				if _, err := client.Get(context.Background(), "hit"); err != nil {
					t.Errorf("Get: %v", err)
				}
			}()
		}
		wg.Wait()
		client.Close()
		fake.waitOpenSocketsEqual(t, 0)
	}

	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	if goroutinesAfter > goroutinesBefore+2 {
		t.Fatalf("goroutines %d -> %d after %d New/use/Close cycles", goroutinesBefore, goroutinesAfter, cycles)
	}
	if got := fake.openSockets(); got != 0 {
		t.Fatalf("open sockets after cycles = %d, want 0", got)
	}
}

// TestCloseDuringHeldGetsReturnsTurns holds N Gets, Closes, releases the hold,
// and asserts idle 0, open sockets 0, and a full in-use-turn channel.
func TestCloseDuringHeldGetsReturnsTurns(t *testing.T) {
	if testing.Short() {
		t.Skip("lifecycle Close-during-inflight stress")
	}
	held := 6
	if raceDetectorOn {
		held = 3
	}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.holdGetsForTest(t)
	client := newTestRedis(t, Config{Host: addr, PoolSize: held, MaxIdleConns: held, IOTimeout: 5 * time.Second, MaxRetries: -1})

	errCh := make(chan error, held)
	for i := 0; i < held; i++ {
		go func() {
			_, err := client.Get(context.Background(), "hit")
			errCh <- err
		}()
	}
	fake.waitHeldGets(t, held)
	client.Close()
	fake.releaseHeldGetsForTest()
	for i := 0; i < held; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("in-flight Get: %v", err)
		}
	}
	if got := pooledIdle(client); got != 0 {
		t.Fatalf("idle after Close = %d, want 0", got)
	}
	fake.waitOpenSocketsEqual(t, 0)
	assertTurnsFullAndNoOverFrees(t, client)
}
