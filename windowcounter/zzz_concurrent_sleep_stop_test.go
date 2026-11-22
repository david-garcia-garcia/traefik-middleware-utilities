package windowcounter

import (
	"context"
	"sync"
	"testing"
	"time"
)

const (
	concurrentSleepStopTimeout = 2 * time.Second
	concurrentSleepStopKey     = "concurrent-sleep-stop"
	concurrentSleepStopLimit   = int64(100)
	concurrentSleepStopWindow  = time.Minute
)

// startHeldFlushConcurrentSleepStop builds a buffered limiter, dirties one key, and holds the next flush EVAL.
func startHeldFlushConcurrentSleepStop(t *testing.T) (limiter *Limiter, release func()) {
	t.Helper()
	fake, addr := startTestFakeRedis(t)
	entered, release := fake.holdNextFlush()
	client := newSimpleRedisForTest(t, addr)
	limiter, err := New(client, minSyncRate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(limiter.Close)
	t.Cleanup(release)
	if _, takeErr := limiter.Take(context.Background(), concurrentSleepStopKey, concurrentSleepStopLimit, concurrentSleepStopWindow); takeErr != nil {
		t.Fatal(takeErr)
	}
	select {
	case <-entered:
	case <-time.After(concurrentSleepStopTimeout):
		t.Fatal("flush EVAL never reached the fake")
	}
	return limiter, release
}

// goSleepConcurrentSleepStop runs Sleep in a goroutine and closes done when it returns.
func goSleepConcurrentSleepStop(limiter *Limiter) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		limiter.Sleep()
	}()
	return done
}

// stoppersConcurrentSleepStop is the limiter stop counter under lock.
func stoppersConcurrentSleepStop(limiter *Limiter) int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.stoppers
}

// flushTimerSetConcurrentSleepStop is true when a flush AfterFunc is stored.
func flushTimerSetConcurrentSleepStop(limiter *Limiter) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.flushTimer != nil
}

// waitBothSleepsWaitingConcurrentSleepStop fails if a Sleep returns before the held EVAL is released, or if both never wait.
func waitBothSleepsWaitingConcurrentSleepStop(t *testing.T, limiter *Limiter, sleep1Done, sleep2Done <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(concurrentSleepStopTimeout)
	for {
		select {
		case <-sleep1Done:
			t.Fatal("Sleep 1 returned before the flush EVAL was released")
		case <-sleep2Done:
			t.Fatal("Sleep 2 returned before the flush EVAL was released")
		case <-time.After(time.Millisecond):
			if stoppersConcurrentSleepStop(limiter) >= 2 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for both Sleeps to wait on the in-flight flush")
			}
		}
	}
}

// waitSleepDoneConcurrentSleepStop fails unless done closes before the timeout.
func waitSleepDoneConcurrentSleepStop(t *testing.T, done <-chan struct{}, which string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(concurrentSleepStopTimeout):
		t.Fatalf("%s did not return after the flush EVAL was released", which)
	}
}

// TestConcurrentSleepStop_TwoSleepsWaitForHeldFlush is Decision 3 (a): neither concurrent Sleep
// returns while the flush EVAL is held; both return after release.
func TestConcurrentSleepStop_TwoSleepsWaitForHeldFlush(t *testing.T) {
	limiter, release := startHeldFlushConcurrentSleepStop(t)
	sleep1Done := goSleepConcurrentSleepStop(limiter)
	sleep2Done := goSleepConcurrentSleepStop(limiter)
	waitBothSleepsWaitingConcurrentSleepStop(t, limiter, sleep1Done, sleep2Done)
	select {
	case <-sleep1Done:
		t.Fatal("Sleep 1 returned before the flush EVAL was released")
	case <-sleep2Done:
		t.Fatal("Sleep 2 returned before the flush EVAL was released")
	default:
	}
	release()
	waitSleepDoneConcurrentSleepStop(t, sleep1Done, "Sleep 1")
	waitSleepDoneConcurrentSleepStop(t, sleep2Done, "Sleep 2")
}

// TestConcurrentSleepStop_TwoSleepsAndWake is Decision 3 (b): Wake during both waits arms nothing;
// after release both Sleeps return; a later Wake then Sleep arms and stops one chain.
func TestConcurrentSleepStop_TwoSleepsAndWake(t *testing.T) {
	limiter, release := startHeldFlushConcurrentSleepStop(t)
	sleep1Done := goSleepConcurrentSleepStop(limiter)
	sleep2Done := goSleepConcurrentSleepStop(limiter)
	waitBothSleepsWaitingConcurrentSleepStop(t, limiter, sleep1Done, sleep2Done)
	limiter.Wake()
	if flushTimerSetConcurrentSleepStop(limiter) {
		t.Fatal("Wake armed a flush timer while Sleeps were waiting")
	}
	select {
	case <-sleep1Done:
		t.Fatal("Sleep 1 returned before the flush EVAL was released")
	case <-sleep2Done:
		t.Fatal("Sleep 2 returned before the flush EVAL was released")
	default:
	}
	release()
	waitSleepDoneConcurrentSleepStop(t, sleep1Done, "Sleep 1")
	waitSleepDoneConcurrentSleepStop(t, sleep2Done, "Sleep 2")
	limiter.Wake()
	if !flushTimerSetConcurrentSleepStop(limiter) {
		t.Fatal("Wake after both Sleeps did not start a flush timer")
	}
	limiter.Sleep()
	if flushTimerSetConcurrentSleepStop(limiter) {
		t.Fatal("Sleep after the final Wake left a flush timer running")
	}
}

// TestConcurrentSleepStop_SingleSleepWakeRace is Decision 3 (c): the live spec's one Sleep versus
// concurrent Wake still returns, and Wake does not arm while that Sleep waits.
func TestConcurrentSleepStop_SingleSleepWakeRace(t *testing.T) {
	limiter, release := startHeldFlushConcurrentSleepStop(t)
	sleepDone := goSleepConcurrentSleepStop(limiter)
	deadline := time.Now().Add(concurrentSleepStopTimeout)
	for stoppersConcurrentSleepStop(limiter) < 1 {
		select {
		case <-sleepDone:
			t.Fatal("Sleep returned before the flush EVAL was released")
		case <-time.After(time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for Sleep to wait on the in-flight flush")
			}
		}
	}
	limiter.Wake()
	if flushTimerSetConcurrentSleepStop(limiter) {
		t.Fatal("Wake armed a flush timer while Sleep was waiting")
	}
	wakeStop := make(chan struct{})
	var wakeWG sync.WaitGroup
	wakeWG.Add(1)
	go func() {
		defer wakeWG.Done()
		for {
			select {
			case <-wakeStop:
				return
			default:
				limiter.Wake()
			}
		}
	}()
	select {
	case <-sleepDone:
		close(wakeStop)
		wakeWG.Wait()
		t.Fatal("Sleep returned before the flush EVAL was released")
	default:
	}
	release()
	waitSleepDoneConcurrentSleepStop(t, sleepDone, "Sleep")
	close(wakeStop)
	wakeWG.Wait()
}
