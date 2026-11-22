package reclaim

import (
	"context"
	"testing"
	"time"
)

// recovered runs call and reports what it panicked with, or nil. It stands in for the plugin
// boundary: under Yaegi a panic is recovered there and the process keeps running, so anything
// the panicking frame left locked stays locked.
func recovered(call func()) (panicValue any) {
	defer func() { panicValue = recover() }()
	call()
	return nil
}

// completes reports whether call returned within budget. A table whose mutex was abandoned
// locked never lets the next caller in, so this is how a wedge shows up.
func completes(call func(), budget time.Duration) bool {
	done := make(chan struct{})
	go func() { defer close(done); call() }()
	select {
	case <-done:
		return true
	case <-time.After(budget):
		return false
	}
}

// TestRepro_PanicUnderTableMutexWedgesTable checks whether a panic raised while t.mu is held
// leaves the table permanently locked. DestBranch released t.mu with a matching Unlock, so a
// panic between lock and unlock skipped that unlock.
//
// Open on a zero-value Table used to reach that shape through the exported API (nil map
// assignment). That path now returns an error; the class is still proven by pre-closing
// slot.ready so put panics on close inside its locked region.
func TestRepro_PanicUnderTableMutexWedgesTable(t *testing.T) {
	h := &recHandler{}

	tabZero := &Table{}
	panicValue := recovered(func() {
		_, _ = tabZero.Open(context.Background(), "k", recLogger(h), func() (any, error) {
			return "v", nil
		}, Hooks{})
	})
	if panicValue != nil {
		if !completes(func() { tabZero.Reset() }, 2*time.Second) {
			t.Fatalf("t.mu was still held after a panic under it (panic: %v): Reset never acquired the mutex, so the table is wedged for the life of the process", panicValue)
		}
	} else if !completes(func() { tabZero.Reset() }, 2*time.Second) {
		t.Fatal("t.mu was still held after Open on a zero-value Table: Reset never acquired the mutex")
	}

	tab := New(Config{Grace: graceNoRace})
	incarnation := &slot{
		state:    slotBusy,
		ready:    make(chan struct{}),
		finished: make(chan struct{}),
		logger:   recLogger(h),
	}
	close(incarnation.ready)
	panicValue = recovered(func() {
		_, _ = tab.put(context.Background(), "k", incarnation, recLogger(h), func() (any, Hooks, error) {
			return "v", Hooks{}, nil
		})
	})
	if panicValue == nil {
		t.Fatal("put did not panic on a pre-closed ready channel; this repro needs a panic inside a lock-held region")
	}
	if !completes(func() {
		if _, err := tab.Open(context.Background(), "other", recLogger(h), func() (any, error) {
			return "v", nil
		}, Hooks{}); err != nil {
			t.Errorf("Open after put panic: %v", err)
		}
		tab.Reset()
	}, 2*time.Second) {
		t.Fatalf("t.mu was still held after a panic under it (panic: %v): a later Open or Reset never acquired the mutex, so the table is wedged for the life of the process", panicValue)
	}
}
