package reclaim

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestPeekMissIsOkFalse(t *testing.T) {
	tab := NewTable(200 * time.Millisecond)
	t.Cleanup(tab.Reset)

	_, _, ok := tab.Peek("missing")
	if ok {
		t.Fatal("Peek of an absent key must be ok=false")
	}
}

func TestPeekBusyIsOkFalseWithoutWaiting(t *testing.T) {
	tab := NewTable(200 * time.Millisecond)
	t.Cleanup(tab.Reset)

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := tab.OpenWithHooks(context.Background(), "busy", slog.Default(), func() (any, Hooks, error) {
			close(started)
			<-release
			return "v", Hooks{}, nil
		})
		done <- err
	}()
	<-started
	peeked := make(chan struct{})
	go func() {
		_, _, busyOK := tab.Peek("busy")
		if busyOK {
			t.Error("Peek of a busy slot must be ok=false")
		}
		close(peeked)
	}()
	select {
	case <-peeked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Peek must not wait on a busy slot")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPeekAwakeDoesNotBind(t *testing.T) {
	tab := NewTable(200 * time.Millisecond)
	t.Cleanup(tab.Reset)

	ctx, cancel := context.WithCancel(context.Background())
	value, err := tab.OpenWithHooks(ctx, "awake", slog.Default(), func() (any, Hooks, error) {
		return "awake-v", Hooks{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, state, ok := tab.Peek("awake")
	if !ok || state != Awake || got != value {
		t.Fatalf("Peek awake: ok=%v state=%v got=%v", ok, state, got)
	}
	cancel()
	if !waitPeekState(t, tab, "awake", Asleep) {
		t.Fatal("cancelling the only Open context must Sleep; Peek must not have bound")
	}
}

func TestPeekAsleepLeavesGraceRunning(t *testing.T) {
	tab := NewTable(200 * time.Millisecond)
	t.Cleanup(tab.Reset)

	closed := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	_, err := tab.OpenWithHooks(ctx, "asleep", slog.Default(), func() (any, Hooks, error) {
		return "asleep-v", Hooks{Close: func() { close(closed) }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !waitPeekState(t, tab, "asleep", Asleep) {
		t.Fatal("last holder gone must Peek Asleep before grace ends")
	}
	select {
	case <-closed:
		t.Fatal("Peek of a sleeping key must not stop grace Close")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("grace must still expire and Close")
	}
	_, _, ok := tab.Peek("asleep")
	if ok {
		t.Fatal("after grace Close Peek must be ok=false")
	}
}

func waitPeekState(t *testing.T, tab *Table, key string, want State) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, state, ok := tab.Peek(key)
		if ok && state == want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
