package reclaim

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	DefaultGrace = 10 * time.Second

	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)

// Hooks are the optional sleep, wake, and close funcs for one incarnation. A nil field skips
// that event. The table stores this value at put and ignores it on a later Open for the same key.
type Hooks struct {
	Sleep func()
	Wake  func()
	Close func()
}

// Table stores one value per key and drives it through create, sleep, wake, and close.
//
//	       Open, key absent
//	              |
//	          create()                 Wake()
//	              v                       |
//	Open ------> AWAKE                    |
//	              |                       |
//	 last holder Done                     |
//	              v                       |
//	          Sleep()                     |
//	              v                       |
//	           ASLEEP ---- Open before ---+
//	              |         grace ends
//	 grace elapsed / Reset / grace == 0
//	              v
//	           Close()  key deleted
//
// Every state change happens under t.mu; create and the stored hooks (Wake, Sleep, Close)
// run outside it with the slot parked in slotBusy. One key's transitions are therefore
// sequential, and a value that is slow to create or slow to sleep never blocks another key. An
// Open or a drop that meets a busy slot waits on slot.ready and looks again.
//
// The goroutine that sees the last holder go Done owns the rest of that incarnation: it sleeps
// the value, writes reclaim_orphan, waits out grace, closes the value, and writes
// reclaim_dispose. Those lines cannot be reordered, because one goroutine writes them in that
// order.
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}

// slotState is what the table may do with a slot right now.
type slotState int

const (
	// slotBusy means create, Wake, or Sleep is in flight. Wait on slot.ready, then look again.
	slotBusy slotState = iota
	// slotAwake means the value is usable and Open may bind a holder to it.
	slotAwake
	// slotAsleep means the value has been slept and is kept until grace ends.
	slotAsleep
	// slotGone means this incarnation has been claimed for close, or create failed. The key is
	// already unmapped, so nothing can reach the slot except a watcher that predates the claim.
	slotGone
)

// slot is one incarnation: the value, what may be done with it, how many holders need it, and
// the create failure it is permanently stuck with if it never got a value.
type slot struct {
	value any
	// hooks are the Sleep, Wake, and Close funcs stored at put. Bind and reclaim do not replace them.
	hooks Hooks
	// createErr is what create returned. Every Open parked on ready replays it, and the slot is
	// gone: a later Open creates a new incarnation rather than retrying this one.
	createErr error
	state     slotState
	holders   int
	// ready is closed when the in-flight transition ends. Waiters re-read state afterwards.
	ready chan struct{}
	// woken is closed by the Open that reclaims a sleeping value, to end its grace wait.
	woken  chan struct{}
	logger *slog.Logger
}

// NewTable builds an empty table. Grace is how long a sleeping value is kept before it is
// disposed. Zero grace keeps nothing. A negative grace becomes DefaultGrace.
func NewTable(grace time.Duration) *Table {
	if grace < 0 {
		grace = DefaultGrace
	}
	return &Table{
		grace: grace,
		items: map[string]*slot{},
	}
}

// requireContext panics if ctx is missing. Traefik New gets a WithCancel ctx; Background is still accepted.
func requireContext(ctx context.Context) {
	if ctx == nil {
		panic("reclaim: Open requires a context")
	}
}

// waitCtx returns when ctx is done. Prefer Done(); if it is nil (Background), poll Err.
func waitCtx(ctx context.Context) {
	if done := ctx.Done(); done != nil {
		<-done
		return
	}
	for ctx.Err() == nil {
		time.Sleep(20 * time.Millisecond)
	}
}

// runSleep runs the stored Sleep hook when it is set. Runs outside t.mu.
func runSleep(hooks Hooks) {
	if hooks.Sleep != nil {
		hooks.Sleep()
	}
}

// runWake runs the stored Wake hook when it is set. Runs outside t.mu, before Open returns.
func runWake(hooks Hooks) {
	if hooks.Wake != nil {
		hooks.Wake()
	}
}

// runClose runs the stored Close hook when it is set. Runs outside t.mu, always after runSleep.
func runClose(hooks Hooks) {
	if hooks.Close != nil {
		hooks.Close()
	}
}

// dispose runs the Close hook and then reports the end, so reclaim_dispose means Close has returned.
func dispose(key string, hooks Hooks, logger *slog.Logger) {
	runClose(hooks)
	logger.Debug(MsgDispose, "key", key)
}

// Open returns the stored value for key, creating it once, and tracks ctx until it is done.
// create takes no arguments: Yaegi cannot call func(context.Context) (any, error).
// logger is required; it is the only logger for this Open and is stored on the slot for orphan
// and dispose. hooks are stored on the incarnation at put; a later Open (bind or reclaim)
// ignores this argument. A sleeping value is woken before Open returns, so a caller never
// receives one asleep. The Close hook, when set, runs when this incarnation ends, after Sleep.
// (value, nil) means this call bound a holder that was still live at return. If ctx.Err() is
// set at bind, Open returns that error and not the pointer, and drops the holder on this call.
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if logger == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil logger", key)
	}
	requireContext(ctx)

	for {
		t.mu.Lock()
		incarnation, mapped := t.items[key]
		if !mapped {
			// Register the key before create runs, so a second first Open waits for this result
			// instead of creating a value that would be thrown away.
			incarnation = &slot{state: slotBusy, ready: make(chan struct{}), logger: logger}
			t.items[key] = incarnation
			t.mu.Unlock()
			return t.put(ctx, key, incarnation, logger, create, hooks)
		}
		switch incarnation.state {
		case slotAwake:
			// Only an Open that binds takes the slot's logger: orphan and dispose belong to the
			// last Open that actually held this incarnation, not to one that merely looked.
			incarnation.logger = logger
			incarnation.holders++
			value := incarnation.value
			t.mu.Unlock()
			logger.Debug(MsgBind, "key", key)
			return t.finishBind(ctx, key, incarnation, value)
		case slotAsleep:
			return t.reclaimLocked(ctx, key, incarnation, logger)
		case slotBusy:
			// A create, wake, or sleep owns the slot. Wait for it, then look again.
			ready := incarnation.ready
			t.mu.Unlock()
			<-ready
			t.mu.Lock()
			createErr := incarnation.createErr
			t.mu.Unlock()
			if createErr != nil {
				return nil, createErr
			}
		case slotGone:
			// This incarnation has ended. Take the key back and create a fresh one.
			if t.items[key] == incarnation {
				delete(t.items, key)
			}
			t.mu.Unlock()
		}
	}
}

// put runs create for a slot this Open registered, then publishes the value or the failure to
// every caller waiting on that slot.
func (t *Table) put(ctx context.Context, key string, incarnation *slot, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error) {
	value, err := create()

	t.mu.Lock()
	if err != nil {
		incarnation.createErr = err
		incarnation.state = slotGone
		if t.items[key] == incarnation {
			delete(t.items, key)
		}
		close(incarnation.ready)
		t.mu.Unlock()
		return nil, err
	}
	incarnation.value = value
	incarnation.hooks = hooks
	incarnation.state = slotAwake
	incarnation.holders++
	// Reset is tests only and must not race Open on a key, but if it did it dropped this slot
	// while create ran. Take the key back rather than strand a value nobody can close.
	t.items[key] = incarnation
	close(incarnation.ready)
	t.mu.Unlock()

	logger.Debug(MsgPut, "key", key)
	logger.Debug(MsgBind, "key", key)
	return t.finishBind(ctx, key, incarnation, value)
}

// reclaimLocked wakes a sleeping slot for this Open and binds ctx. The caller holds t.mu and has
// seen slotAsleep; this releases it, because Wake must not run under the table mutex. If ctx is
// already done after wake, it drops this holder and returns that error without the pointer.
func (t *Table) reclaimLocked(ctx context.Context, key string, incarnation *slot, logger *slog.Logger) (any, error) {
	incarnation.logger = logger
	incarnation.state = slotBusy
	incarnation.ready = make(chan struct{})
	if incarnation.woken != nil {
		// End the grace wait: this incarnation is not being disposed after all.
		close(incarnation.woken)
		incarnation.woken = nil
	}
	incarnation.holders++
	value := incarnation.value
	storedHooks := incarnation.hooks
	t.mu.Unlock()

	runWake(storedHooks)

	t.mu.Lock()
	incarnation.state = slotAwake
	close(incarnation.ready)
	t.mu.Unlock()

	logger.Debug(MsgReclaim, "key", key)
	logger.Debug(MsgBind, "key", key)
	return t.finishBind(ctx, key, incarnation, value)
}

// finishBind returns the bound value if ctx is still live, and watches it until Done. If ctx is
// already done, it drops this holder now (no AfterFunc) and returns that error without the pointer.
func (t *Table) finishBind(ctx context.Context, key string, incarnation *slot, value any) (any, error) {
	if err := ctx.Err(); err != nil {
		t.drop(key, incarnation)
		return nil, err
	}
	t.dropWhenDone(key, incarnation, ctx)
	return value, nil
}

// dropWhenDone runs drop when ctx is done. A holder with a Done channel uses AfterFunc so the
// hold does not park a waiter. A holder whose Done is nil still needs watch to poll Err.
func (t *Table) dropWhenDone(key string, incarnation *slot, ctx context.Context) {
	if ctx.Done() != nil {
		context.AfterFunc(ctx, func() { t.drop(key, incarnation) })
		return
	}
	go t.watch(key, incarnation, ctx)
}

// watch waits until ctx is done, then drops that holder from this slot.
func (t *Table) watch(key string, incarnation *slot, ctx context.Context) {
	waitCtx(ctx)
	t.drop(key, incarnation)
}

// drop removes one holder. When it was the last one, this goroutine ends the incarnation: sleep,
// orphan, grace, close, dispose — in that order, so those lines cannot be reordered. A watcher
// whose incarnation is already gone finds slotGone and returns.
func (t *Table) drop(key string, incarnation *slot) {
	t.mu.Lock()
	incarnation.holders--
	for incarnation.state == slotBusy {
		// A create, wake, or sleep owns the slot. Wait for it before deciding to sleep.
		ready := incarnation.ready
		t.mu.Unlock()
		<-ready
		t.mu.Lock()
	}
	if incarnation.holders > 0 || incarnation.state != slotAwake {
		t.mu.Unlock()
		return
	}

	incarnation.state = slotBusy
	incarnation.ready = make(chan struct{})
	incarnation.woken = make(chan struct{})
	woken := incarnation.woken
	logger := incarnation.logger
	storedHooks := incarnation.hooks
	grace := t.grace
	t.mu.Unlock()

	runSleep(storedHooks)
	// Orphan is written while the slot is still busy. Reset leaves a busy slot to the goroutine
	// that owns the transition, so nothing else can write this incarnation's dispose line first.
	logger.Debug(MsgOrphan, "key", key)

	t.mu.Lock()
	incarnation.state = slotAsleep
	close(incarnation.ready)
	mapped := t.items[key] == incarnation
	if mapped && grace <= 0 {
		// Zero grace keeps nothing: unmap now, so no Open can ever see this value asleep.
		delete(t.items, key)
		mapped = false
	}
	t.mu.Unlock()

	// Not mapped means zero grace, or Reset dropped this slot: either way it is ours to close.
	if !mapped {
		t.expire(key, incarnation)
		return
	}

	go t.waitGraceOrWake(key, incarnation, woken, grace)
}

// waitGraceOrWake closes the incarnation when grace elapses, unless a reclaim woke it. The timer
// wait is not on the drop caller: a canceled bind must not stall Open for the grace duration.
func (t *Table) waitGraceOrWake(key string, incarnation *slot, woken chan struct{}, grace time.Duration) {
	wait := time.NewTimer(grace)
	defer wait.Stop()
	select {
	case <-wait.C:
		t.expire(key, incarnation)
	case <-woken:
	}
}

// expire ends a sleeping incarnation, unless an Open woke it or something else already claimed
// it. It is the only place that closes a value the table still had mapped.
func (t *Table) expire(key string, incarnation *slot) {
	t.mu.Lock()
	if incarnation.state != slotAsleep || incarnation.holders > 0 {
		t.mu.Unlock()
		return
	}
	incarnation.state = slotGone
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	logger := incarnation.logger
	storedHooks := incarnation.hooks
	t.mu.Unlock()
	dispose(key, storedHooks, logger)
}

// Reset ends every incarnation on this table. An awake value is slept first, so Close never sees
// a live value and orphan still precedes dispose. Tests only: it must not race an Open on the
// same key. A slot that is mid-transition is ended by the goroutine that owns that transition.
func (t *Table) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	items := t.items
	t.items = map[string]*slot{}
	for _, incarnation := range items {
		if incarnation.woken != nil {
			close(incarnation.woken)
			incarnation.woken = nil
		}
	}
	t.mu.Unlock()

	for key, incarnation := range items {
		t.mu.Lock()
		state, storedHooks, logger := incarnation.state, incarnation.hooks, incarnation.logger
		if state == slotAwake || state == slotAsleep {
			incarnation.state = slotGone
		}
		t.mu.Unlock()

		switch state {
		case slotAwake:
			runSleep(storedHooks)
			logger.Debug(MsgOrphan, "key", key)
			dispose(key, storedHooks, logger)
		case slotAsleep:
			dispose(key, storedHooks, logger)
		case slotBusy, slotGone:
			// The goroutine that owns this transition ends the incarnation itself: it finds the
			// slot unmapped, or its grace wait already released by the loop above.
		}
	}
}
