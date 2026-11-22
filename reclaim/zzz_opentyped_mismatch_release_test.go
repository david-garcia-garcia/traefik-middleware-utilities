package reclaim

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// storedStringMismatchRelease is the value create stores so OpenTyped[int] mismatches.
const storedStringMismatchRelease = "string-value"

// TestMismatchRelease_ZeroGraceCloseRunsWhileCallerCtxLive is Decision 5 (a): a creating
// OpenTyped mismatch at grace 0 Closes once and unmaps the key while the caller's ctx is live.
func TestMismatchRelease_ZeroGraceCloseRunsWhileCallerCtxLive(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	var closes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := OpenTyped[int](ctx, tab, "k", recLogger(h), func() (any, Hooks, error) {
		return storedStringMismatchRelease, Hooks{Close: func() { closes.Add(1) }}, nil
	})
	if got != 0 {
		t.Fatalf("mismatch value %v, want 0", got)
	}
	want := fmt.Sprintf("reclaim: open %q: want %T, got %T", "k", 0, storedStringMismatchRelease)
	if err == nil || err.Error() != want {
		t.Fatalf("mismatch err %v, want %s", err, want)
	}
	waitUntil(t, func() bool { return closes.Load() == 1 })
	if mappedKeys(tab) != 0 {
		t.Fatalf("key still mapped after mismatch at grace 0")
	}
	if ctx.Err() != nil {
		t.Fatalf("caller ctx already done: %v", ctx.Err())
	}
	if n := closes.Load(); n != 1 {
		t.Fatalf("Close ran %d times, want 1", n)
	}
}

// TestMismatchRelease_HundredFailedCallsKeepOneLiveHolder is Decision 5 (b): 100 failed
// OpenTyped calls on a key another holder keeps leave holders at 1; Close runs once after
// that holder is canceled.
func TestMismatchRelease_HundredFailedCallsKeepOneLiveHolder(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	var closes atomic.Int32
	keep, keepCancel := context.WithCancel(context.Background())
	defer keepCancel()
	if _, err := tab.Open(keep, "k", recLogger(h), func() (any, error) {
		return storedStringMismatchRelease, nil
	}, Hooks{Close: func() { closes.Add(1) }}); err != nil {
		t.Fatalf("keep open: %v", err)
	}
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		_, err := OpenTyped[int](ctx, tab, "k", recLogger(h), func() (any, Hooks, error) {
			t.Error("create ran on mismatch retry")
			return "no", Hooks{}, nil
		})
		cancel()
		if err == nil {
			t.Fatal("mismatch err nil")
		}
	}
	incarnation := mustSlot(t, tab, "k")
	if n := holderCount(tab, incarnation); n != 1 {
		t.Fatalf("holders=%d, want 1", n)
	}
	if n := closes.Load(); n != 0 {
		t.Fatalf("Close ran while keep holder live: %d", n)
	}
	keepCancel()
	waitUntil(t, func() bool { return closes.Load() == 1 })
	if n := closes.Load(); n != 1 {
		t.Fatalf("Close ran %d times after keep cancel, want 1", n)
	}
}

// TestMismatchRelease_CancelAfterMismatchDoesNotDropTwice is Decision 5 (c): cancelling the
// failed caller's context after the mismatch release does not drop again.
func TestMismatchRelease_CancelAfterMismatchDoesNotDropTwice(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	var closes atomic.Int32
	keep, keepCancel := context.WithCancel(context.Background())
	defer keepCancel()
	if _, err := tab.Open(keep, "k", recLogger(h), func() (any, error) {
		return storedStringMismatchRelease, nil
	}, Hooks{Close: func() { closes.Add(1) }}); err != nil {
		t.Fatalf("keep open: %v", err)
	}
	failed, failedCancel := context.WithCancel(context.Background())
	defer failedCancel()
	_, err := OpenTyped[int](failed, tab, "k", recLogger(h), func() (any, Hooks, error) {
		t.Error("create ran on mismatch bind")
		return "no", Hooks{}, nil
	})
	if err == nil {
		t.Fatal("mismatch err nil")
	}
	incarnation := mustSlot(t, tab, "k")
	if n := holderCount(tab, incarnation); n != 1 {
		t.Fatalf("holders after mismatch=%d, want 1", n)
	}
	failedCancel()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		n := holderCount(tab, incarnation)
		if n < 0 {
			t.Fatalf("holders went negative: %d", n)
		}
		if n != 1 {
			t.Fatalf("holders after failed-ctx cancel=%d, want 1", n)
		}
		time.Sleep(time.Millisecond)
	}
	if n := closes.Load(); n != 0 {
		t.Fatalf("Close ran after failed-ctx cancel: %d", n)
	}
	keepCancel()
	waitUntil(t, func() bool { return closes.Load() == 1 })
	if n := closes.Load(); n != 1 {
		t.Fatalf("Close ran %d times, want 1", n)
	}
}

// TestMismatchRelease_NilDoneContextReleasesWithoutLeftoverWatch is Decision 5 (d): a
// context.Background() mismatch is released and leaves no watcher once the incarnation ends.
func TestMismatchRelease_NilDoneContextReleasesWithoutLeftoverWatch(t *testing.T) {
	const slack = 2
	base := settledGoroutines()
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	var closes atomic.Int32
	_, err := OpenTyped[int](context.Background(), tab, "k", recLogger(h), func() (any, Hooks, error) {
		return storedStringMismatchRelease, Hooks{Close: func() { closes.Add(1) }}, nil
	})
	if err == nil {
		t.Fatal("mismatch err nil")
	}
	waitUntil(t, func() bool { return closes.Load() == 1 && mappedKeys(tab) == 0 })
	deadline := time.Now().Add(leakBudget)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+slack {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%d goroutines after mismatch release, baseline %d slack %d", runtime.NumGoroutine(), base, slack)
}

// TestMismatchRelease_SuccessfulOpenTypedKeepsHolder is Decision 5 (e): a matching OpenTyped
// still binds; Close waits for the caller's context.
func TestMismatchRelease_SuccessfulOpenTypedKeepsHolder(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	life := &lifecycle{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := OpenTyped[*lifecycle](ctx, tab, "a", recLogger(h), func() (any, Hooks, error) {
		return life, testLifeHooks(life), nil
	})
	if err != nil {
		t.Fatalf("OpenTyped: %v", err)
	}
	if got != life {
		t.Fatalf("OpenTyped value %v, want the created lifecycle", got)
	}
	incarnation := mustSlot(t, tab, "a")
	if n := holderCount(tab, incarnation); n != 1 {
		t.Fatalf("holders=%d, want 1", n)
	}
	if life.count("close") != 0 {
		t.Fatal("Close ran while the matching OpenTyped holder is live")
	}
	cancel()
	waitUntil(t, func() bool { return life.count("close") == 1 })
}
