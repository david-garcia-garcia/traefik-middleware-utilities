package reclaim

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// TestWaitCtx_DoneChannelBranch pins what waitCtx does for a holder that has a Done channel.
// The table never reaches this branch: dropWhenDone starts watch only when ctx.Done() is nil,
// and watch is the only caller. It is covered here so the branch is not merely unexercised.
func TestWaitCtx_DoneChannelBranch(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if waitCtx(canceled, make(chan struct{})) {
		t.Fatal("a canceled holder was reported as an ended incarnation; the caller would skip its drop")
	}

	live, cancelLive := context.WithCancel(context.Background())
	defer cancelLive()
	ended := make(chan struct{})
	close(ended)
	if !waitCtx(live, ended) {
		t.Fatal("an ended incarnation was not reported; the caller would drop a holder from a gone slot")
	}
}

// TestWaitCtx_PollingSeesAnEndedIncarnationFirst covers the non-blocking check at the top of the
// polling loop. It is the only branch that answers a holder whose incarnation had already ended
// before the watcher looked, without waiting out a 20ms tick first.
func TestWaitCtx_PollingSeesAnEndedIncarnationFirst(t *testing.T) {
	ended := make(chan struct{})
	close(ended)
	if !waitCtx(&nilDoneCtx{}, ended) {
		t.Fatal("a polling watcher did not report an already-ended incarnation")
	}
}

// TestTable_ResetSleepPanicStillDisposes covers Reset's Sleep-panic branch. devdocs says a Sleep
// panic during Reset skips orphan and still disposes, which nothing exercised.
func TestTable_ResetSleepPanicStillDisposes(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var closes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Sleep: func() { panic("sleep boom") },
		Close: func() { closes.Add(1) },
	}); err != nil {
		t.Fatalf("open: %v", err)
	}

	// Reset sleeps an awake value on its own stack, so everything has happened when it returns.
	tab.Reset()

	if got := closes.Load(); got != 1 {
		t.Fatalf("Close ran %d times after a Sleep panic in Reset, want 1", got)
	}
	// Sleep never returned, so the value was never parked asleep and orphan must not be claimed.
	if n := countKeyMsg(h.events(), MsgOrphan, "a"); n != 0 {
		t.Fatalf("%d orphan lines after a Sleep panic in Reset, want 0", n)
	}
	if n := countKeyMsg(h.events(), MsgDispose, "a"); n != 1 {
		t.Fatalf("%d dispose lines, want 1", n)
	}
	lines := h.hookPanics()
	if len(lines) != 1 {
		t.Fatalf("%d hook panic lines, want 1", len(lines))
	}
	if lines[0].level != slog.LevelError || lines[0].hook != "sleep" || lines[0].key != "a" {
		t.Fatalf("hook panic line %+v, want key a hook sleep at error", lines[0])
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("Reset left a key stored after a Sleep panic")
	}
}

// TestTable_ResetUnmapsBeforeCloseWithEnforce locks the documented exception: Reset unmaps first
// regardless of the stored EnforceCloseBeforeOpen, so a later Open creates instead of waiting for
// that Close. No test combined Reset with the flag.
func TestTable_ResetUnmapsBeforeCloseWithEnforce(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	releaseOnce(t, releaseClose)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}

	// Reset runs Close on its own stack, so hold it there and look at the table meanwhile.
	resetDone := make(chan struct{})
	go func() { defer close(resetDone); tab.Reset() }()
	select {
	case <-closeEntered:
	case <-time.After(waitBudget):
		t.Fatal("Reset did not reach Close")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	value, err := openReturned(ctx2, t, tab, "k", recLogger(h), func() (any, error) {
		return nextIncarnation, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if value != nextIncarnation {
		t.Fatalf("Open value %v, want a fresh incarnation: Reset unmaps before Close even with the flag set", value)
	}

	close(releaseClose)
	<-resetDone
}

// TestTable_ExpireLeavesAClaimedIncarnation covers expire's guard. expire is an armed AfterFunc,
// so it can arrive after an Open already woke the value or after another path claimed it. Both
// shapes are reached here directly, because winning that timer race on purpose is not reliable.
func TestTable_ExpireLeavesAClaimedIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var closes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Close: func() { closes.Add(1) },
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "a")
	incarnation := mustSlot(t, tab, "a")

	// An Open that reclaims parks the slot busy before a late expire can claim it.
	tab.mu.Lock()
	incarnation.state = slotBusy
	tab.mu.Unlock()
	tab.expire("a", incarnation)
	if got := closes.Load(); got != 0 {
		t.Fatalf("expire closed a value that was mid-transition (%d closes)", got)
	}

	// A holder that bound during grace protects it too.
	tab.mu.Lock()
	incarnation.state = slotAsleep
	incarnation.holders = 1
	tab.mu.Unlock()
	tab.expire("a", incarnation)
	if got := closes.Load(); got != 0 {
		t.Fatalf("expire closed a value that still had a holder (%d closes)", got)
	}
	if n := countKeyMsg(h.events(), MsgDispose, "a"); n != 0 {
		t.Fatalf("%d dispose lines from an expire that should have returned, want 0", n)
	}

	tab.mu.Lock()
	incarnation.holders = 0
	tab.mu.Unlock()
	tab.Reset()
}

// TestTable_OpenReplacesAMappedGoneIncarnation covers Open's slotGone branch. Every path that
// ends an incarnation unmaps or replaces the key in the same t.mu hold, except Reset: it swaps
// the map and only then ends each slot, so a put that maps its slot back in between leaves a
// mapped, gone incarnation. That window is not reliably schedulable, so the shape is built here.
func TestTable_OpenReplacesAMappedGoneIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	gone := &slot{state: slotGone, ready: make(chan struct{}), logger: recLogger(h)}
	close(gone.ready)
	tab.mu.Lock()
	tab.items["a"] = gone
	tab.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	value, err := openReturned(ctx, t, tab, "a", recLogger(h), func() (any, error) {
		return unstuckValue, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("open over a gone incarnation: %v", err)
	}
	if value != unstuckValue {
		t.Fatalf("Open value %v, want a fresh incarnation", value)
	}
	if got := mustSlot(t, tab, "a"); got == gone {
		t.Fatal("Open kept the gone incarnation mapped")
	}
	if n := countKeyMsg(h.events(), MsgPut, "a"); n != 1 {
		t.Fatalf("%d put lines, want 1", n)
	}
}

// TestTable_DropWaitsForAnInFlightTransition covers drop's busy-wait loop. A holder keeps the
// count above zero, so no ordinary schedule lets a drop meet a busy slot; the state is set here
// to prove the loop waits for the transition instead of sleeping a value it does not own.
func TestTable_DropWaitsForAnInFlightTransition(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	keep, keepCancel := context.WithCancel(context.Background())
	defer keepCancel()
	if _, err := tab.Open(keep, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}
	incarnation := mustSlot(t, tab, "a")

	// Two holders and a transition in flight: the drop under test must park on ready.
	ready := make(chan struct{})
	tab.mu.Lock()
	incarnation.holders = 2
	incarnation.state = slotBusy
	incarnation.ready = ready
	tab.mu.Unlock()

	dropped := make(chan struct{})
	go func() { defer close(dropped); tab.drop("a", incarnation) }()
	select {
	case <-dropped:
		t.Fatal("drop returned while a transition was in flight")
	case <-time.After(50 * time.Millisecond):
	}

	// End the transition the way the owning goroutine would.
	tab.mu.Lock()
	incarnation.state = slotAwake
	tab.mu.Unlock()
	close(ready)
	select {
	case <-dropped:
	case <-time.After(waitBudget):
		t.Fatal("drop never returned after the transition ended")
	}

	if got := life.count("sleep"); got != 0 {
		t.Fatalf("sleep ran %d times, want 0: a holder still had the value", got)
	}
	if got := holderCount(tab, incarnation); got != 1 {
		t.Fatalf("holders %d, want 1", got)
	}
}
