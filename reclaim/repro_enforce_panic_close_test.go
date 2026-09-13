package reclaim

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// releaseOnce closes ch unless it is already closed. The Close hooks in this file block until the
// test releases them; a test that fails early must still let that hook return.
func releaseOnce(t *testing.T, ch chan struct{}) {
	t.Helper()
	t.Cleanup(func() {
		select {
		case <-ch:
		default:
			close(ch)
		}
	})
}

// TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce checks whether the Sleep-panic ending path
// honours the stored EnforceCloseBeforeOpen. drop recovers that panic through endBusySlot, which
// unmaps the key and closes ready before dispose runs Close, so a racing Open creates a second
// incarnation while the first one's Close still holds the exclusive resource the flag exists for.
func TestRepro_SleepPanicUnmapsBeforeCloseWithEnforce(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	releaseOnce(t, releaseClose)
	var createWhileCloseBlocked atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Sleep:                  func() { panic("sleep boom") },
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	// A Sleep panic ends the incarnation, so Close runs even though Sleep never returned. Hold it.
	select {
	case <-closeEntered:
	case <-time.After(waitBudget):
		t.Fatal("Close did not run after the Sleep panic")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			createWhileCloseBlocked.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		opened <- err
	}()
	// The ending incarnation stored EnforceCloseBeforeOpen, so this Open must stay parked for the
	// whole Close instead of creating a value that holds the same exclusive resource twice.
	select {
	case err := <-opened:
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
		t.Fatalf("second Open returned before Close returned: %v", err)
	case <-time.After(200 * time.Millisecond):
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
	}

	close(releaseClose)
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
	case <-time.After(waitBudget):
		t.Fatal("second Open did not return after Close returned")
	}
	if !createWhileCloseBlocked.Load() {
		t.Fatal("second Open did not create after Close returned")
	}
}

// TestRepro_WakePanicUnmapsBeforeCloseWithEnforce is the same gap on the Wake-panic ending path.
// reclaimLocked recovers the panic through endBusySlot (unmap, close ready) and only then runs
// dispose, so the key is already free for a new create while Close is in flight.
func TestRepro_WakePanicUnmapsBeforeCloseWithEnforce(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	releaseOnce(t, releaseClose)
	var createWhileCloseBlocked atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Wake:                   func() { panic("wake boom") },
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	// Wait for the orphan line: an Open before it is a plain second bind, not the wake branch.
	waitKeyMsg(t, h, MsgOrphan, "k")

	// This Open reclaims the sleeping value, the stored Wake panics, and reclaimLocked runs the
	// ending Close on this stack before it returns the wrapped panic to its caller.
	ctxWake, cancelWake := context.WithCancel(context.Background())
	defer cancelWake()
	woke := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctxWake, "k", recLogger(h), func() (any, error) { return "unreachable", nil }, Hooks{})
		woke <- err
	}()
	select {
	case <-closeEntered:
	case <-time.After(waitBudget):
		t.Fatal("Close did not run after the Wake panic")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			createWhileCloseBlocked.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		opened <- err
	}()
	select {
	case err := <-opened:
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
		t.Fatalf("second Open returned before Close returned: %v", err)
	case <-time.After(200 * time.Millisecond):
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
	}

	close(releaseClose)
	// Both Opens must be back before this test returns, or they log into a finished test.
	select {
	case err := <-woke:
		if err == nil {
			t.Fatal("the reclaiming Open returned no error for a panicking Wake")
		}
	case <-time.After(waitBudget):
		t.Fatal("the reclaiming Open did not return after Close returned")
	}
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
	case <-time.After(waitBudget):
		t.Fatal("second Open did not return after Close returned")
	}
	if !createWhileCloseBlocked.Load() {
		t.Fatal("second Open did not create after Close returned")
	}
}
