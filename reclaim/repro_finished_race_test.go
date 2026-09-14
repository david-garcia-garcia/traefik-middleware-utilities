package reclaim

import (
	"context"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// slowErrNilDone is a nil-Done holder whose first Err() call is slow. finishBind calls Err()
// immediately before dropWhenDone reads slot.finished, so this parks the binding Open in that
// exact gap without taking any lock the ending goroutine also takes.
type slowErrNilDone struct {
	delay   time.Duration
	slowed  atomic.Bool
	entered chan struct{}
}

// Deadline reports no deadline.
func (c *slowErrNilDone) Deadline() (time.Time, bool) { return time.Time{}, false }

// Done returns nil, which is what routes this holder to the polling watcher.
func (c *slowErrNilDone) Done() <-chan struct{} { return nil }

// Err parks the first caller for delay and then reports a live holder forever.
func (c *slowErrNilDone) Err() error {
	if c.slowed.CompareAndSwap(false, true) {
		close(c.entered)
		time.Sleep(c.delay)
	}
	return nil
}

// Value has nothing to look up.
func (c *slowErrNilDone) Value(any) any { return nil }

// TestRepro_FinishedReadRacesReset checks whether dropWhenDone reads slot.finished without
// holding t.mu while closeFinished writes that field under it. A holder whose Done() is nil is
// the only bind shape that reads the field, and Reset is the only ending path that can close a
// still-awake incarnation, so the two together are what -race needs to see the read.
func TestRepro_FinishedReadRacesReset(t *testing.T) {
	const rounds = 20
	for round := 0; round < rounds; round++ {
		h := &recHandler{}
		tab := New(Config{Grace: graceNoRace})
		key := "k" + strconv.Itoa(round)

		// A live holder keeps the incarnation awake, so the second Open takes the bind path.
		keep, keepCancel := context.WithCancel(context.Background())
		if _, err := tab.Open(keep, key, recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("round %d keep-alive open: %v", round, err)
		}

		holder := &slowErrNilDone{delay: 50 * time.Millisecond, entered: make(chan struct{})}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = tab.Open(holder, key, recLogger(h), func() (any, error) {
				t.Error("create ran again")
				return nil, nil
			}, Hooks{})
		}()
		go func() {
			defer wg.Done()
			// Reset only after that Open released t.mu and parked inside ctx.Err(), so the
			// write to finished lands before the unsynchronized read of it.
			<-holder.entered
			tab.Reset()
		}()
		wg.Wait()

		keepCancel()
		tab.Reset()
	}
}

// TestRepro_FinishedReadRaceLeaksWatcher is what the race at table.go:414 costs. When Reset nils
// finished before dropWhenDone reads it, the watcher is started with a nil channel: waitCtx then
// selects on nil forever and only cancellation can stop it. A holder that is never canceled
// therefore polls for the life of the process even though its incarnation has already ended,
// which is the leak repro_nildone_watcher_test.go says is fixed.
func TestRepro_FinishedReadRaceLeaksWatcher(t *testing.T) {
	const keys = 20
	const slack = 2
	base := settledGoroutines()

	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})

	for i := 0; i < keys; i++ {
		key := "k" + strconv.Itoa(i)
		// A live holder keeps the incarnation awake, so the next Open takes the bind path.
		keep, keepCancel := context.WithCancel(context.Background())
		if _, err := tab.Open(keep, key, recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("keep-alive open %d: %v", i, err)
		}

		holder := &slowErrNilDone{delay: 200 * time.Millisecond, entered: make(chan struct{})}
		opened := make(chan struct{})
		go func() {
			defer close(opened)
			if _, err := tab.Open(holder, key, recLogger(h), func() (any, error) {
				t.Error("create ran again")
				return nil, nil
			}, Hooks{}); err != nil {
				t.Errorf("nil-Done open %d: %v", i, err)
			}
		}()

		// End the incarnation while that Open is parked in ctx.Err(), before it reads finished.
		<-holder.entered
		tab.Reset()
		waitKeyMsg(t, h, MsgDispose, key)
		<-opened
		keepCancel()
	}

	// Every incarnation has ended, so no watcher has a holder left to drop. A watcher that
	// captured a nil finished cannot see that and keeps polling.
	deadline := time.Now().Add(leakBudget)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+slack {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%d goroutines after every incarnation ended, baseline %d and slack %d: %d watchers that read a nil finished are still polling",
		runtime.NumGoroutine(), base, slack, keys)
}
