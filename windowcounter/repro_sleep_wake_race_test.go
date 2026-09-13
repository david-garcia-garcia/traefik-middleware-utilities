package windowcounter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRepro_SleepWakeRaceHangs checks whether Wake can startFlushLocked after
// Sleep's stopFlushAndWait has set stop=nil and unlocked, but before wg.Wait.
// That Add(1) of a new flushLoop makes Sleep Wait for a loop nobody will stop.
func TestRepro_SleepWakeRaceHangs(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	defer limiter.Close()

	const iterations = 80
	const sleepTimeout = 2 * time.Second
	const wakeWorkers = 8

	var waitGroupPanic atomic.Bool
	var panicText atomic.Value

	for i := 0; i < iterations; i++ {
		sleepDone := make(chan struct{})
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					waitGroupPanic.Store(true)
					panicText.Store(recovered)
				}
				close(sleepDone)
			}()
			limiter.Sleep()
		}()

		wakeStop := make(chan struct{})
		var wakeWG sync.WaitGroup
		for w := 0; w < wakeWorkers; w++ {
			wakeWG.Add(1)
			go func() {
				defer wakeWG.Done()
				defer func() {
					if recovered := recover(); recovered != nil {
						waitGroupPanic.Store(true)
						panicText.Store(recovered)
					}
				}()
				for {
					select {
					case <-wakeStop:
						return
					default:
						limiter.Wake()
					}
				}
			}()
		}

		select {
		case <-sleepDone:
			close(wakeStop)
			wakeWG.Wait()
		case <-time.After(sleepTimeout):
			close(wakeStop)
			wakeWG.Wait()
			t.Fatalf("Sleep hung on iteration %d after %v", i, sleepTimeout)
		}

		if waitGroupPanic.Load() {
			t.Fatalf("WaitGroup panic on iteration %d: %v", i, panicText.Load())
		}
	}
}
