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

	MsgPut       = "reclaim_put"
	MsgBind      = "reclaim_bind"
	MsgOrphan    = "reclaim_orphan"
	MsgReclaim   = "reclaim_reclaim"
	MsgDispose   = "reclaim_dispose"
	MsgHookPanic = "reclaim_hook_panic"
)

// Hooks are the optional sleep, wake, and close funcs for one incarnation, plus whether that
// incarnation keeps the key mapped until Close returns. A nil func skips that event. The table
// stores this value at put and ignores it on a later Open for the same key.
type Hooks struct {
	Sleep func()
	Wake  func()
	Close func()
	// EnforceCloseBeforeOpen keeps this incarnation mapped slotBusy until Close returns, so a
	// later Open of the same key waits and then creates. The zero value (false) unmaps first: a
	// concurrent Open may create while Close is still in flight. Set this when the value owns
	// something exclusive that cannot be held twice. The ending path reads the stored hooks, not
	// a later Open's argument.
	EnforceCloseBeforeOpen bool
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
// run outside it. Wake and Sleep park the slot in slotBusy. Close does too when the stored
// EnforceCloseBeforeOpen is set; otherwise the key is unmapped first, so a concurrent Open
// may create during Close. An Open or a drop that meets a busy slot waits on slot.ready and
// looks again.
//
// The last-holder drop sleeps the value and writes reclaim_orphan, then either expires at zero
// grace or starts waitGraceOrWake. Close and reclaim_dispose stay after orphan, in that order.
// A Sleep panic aborts instead: Close and unmap, no orphan. Those lines cannot be reordered,
// because one goroutine writes them in that order.
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}

// slotState is what the table may do with a slot right now.
type slotState int

const (
	// slotBusy means create, Wake, Sleep, or (when EnforceCloseBeforeOpen) Close is in flight.
	// Wait on slot.ready, then look again.
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

// Config is the freeze-at-New settings for a Table. New copies Grace onto the table.
// Later writes to this struct do not change a table that already ran New.
type Config struct {
	// Grace is how long a sleeping value is kept before it is disposed. Zero keeps nothing.
	// A negative Grace becomes DefaultGrace.
	Grace time.Duration
}

// New builds an empty table. Grace is copied from cfg and MUST NOT change on that table afterwards.
func New(cfg Config) *Table {
	grace := cfg.Grace
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

// runHook calls one hook and returns what it panicked with, or nil when it is unset or returned.
// The table finishes the slotBusy protocol either way: a hook runs on an AfterFunc goroutine, so
// letting the panic out would kill the process. Returning it keeps the caller in charge of the
// failure — no caller may treat nil-or-not as success. Runs outside t.mu.
func runHook(hook func()) (recovered any) {
	if hook == nil {
		return nil
	}
	defer func() { recovered = recover() }()
	hook()
	return nil
}

// dispose runs the Close hook and then reports the end. reclaim_dispose means Close has returned;
// a Close panic is reported on its own line, because nothing can retry. Close is invoked only
// here, through runHook, so an AfterFunc panic cannot kill the process on either ending path.
func dispose(key string, hooks Hooks, logger *slog.Logger) {
	if recovered := runHook(hooks.Close); recovered != nil {
		logger.Error(MsgHookPanic, "key", key, "hook", "close", "panic", recovered)
	}
	logger.Debug(MsgDispose, "key", key)
}

// endBusySlot records createErr on a busy slot (nil means waiters create), unmaps it, and closes ready.
func (t *Table) endBusySlot(key string, incarnation *slot, createErr error) {
	t.mu.Lock()
	incarnation.createErr = createErr
	incarnation.state = slotGone
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	close(incarnation.ready)
	t.mu.Unlock()
}

// unmapAfterClose unmaps a still-mapped busy incarnation after Close has returned (or its panic
// was recovered) and closes ready so waiters create. The caller already ran dispose outside t.mu.
func (t *Table) unmapAfterClose(key string, incarnation *slot) {
	t.mu.Lock()
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	incarnation.state = slotGone
	ready := incarnation.ready
	t.mu.Unlock()
	close(ready)
}

// endMappedClose runs dispose while the slot is still mapped slotBusy, then unmaps. Used when
// the stored EnforceCloseBeforeOpen is set. Close never runs under t.mu.
func (t *Table) endMappedClose(key string, incarnation *slot, storedHooks Hooks, logger *slog.Logger) {
	dispose(key, storedHooks, logger)
	t.unmapAfterClose(key, incarnation)
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
	if create == nil {
		return nil, fmt.Errorf("reclaim: create %q: nil create", key)
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
			// A create, wake, sleep, or (when EnforceCloseBeforeOpen) close owns the slot.
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
	var value any
	var err error
	if recovered := runHook(func() { value, err = create() }); recovered != nil {
		err = fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)
	}
	if err != nil {
		t.endBusySlot(key, incarnation, err)
		return nil, err
	}

	t.mu.Lock()
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

	if recovered := runHook(storedHooks.Wake); recovered != nil {
		// This Open has a caller to answer, so it reports the panic as its error. A waiter parked
		// on ready replays it from createErr rather than resuming a value Wake left half-done.
		err := fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)
		t.endBusySlot(key, incarnation, err)
		dispose(key, storedHooks, logger)
		return nil, err
	}

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

// drop removes one holder. When it was the last one, this goroutine sleeps the value and writes
// reclaim_orphan. Zero grace expires on this stack. Positive grace continues in waitGraceOrWake.
// Orphan still precedes dispose. A Sleep panic aborts: Close, unmap, no orphan. When the stored
// EnforceCloseBeforeOpen is set, Close runs while the key is still mapped slotBusy. A watcher
// whose incarnation is already gone finds slotGone and returns.
func (t *Table) drop(key string, incarnation *slot) {
	t.mu.Lock()
	incarnation.holders--
	for incarnation.state == slotBusy {
		// A create, wake, sleep, or (when EnforceCloseBeforeOpen) close owns the slot.
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

	if recovered := runHook(storedHooks.Sleep); recovered != nil {
		// Nobody is waiting on a return value here, so createErr stays nil: a later Open creates a
		// fresh incarnation instead of inheriting one whose Sleep never finished.
		logger.Error(MsgHookPanic, "key", key, "hook", "sleep", "panic", recovered)
		t.endBusySlot(key, incarnation, nil)
		dispose(key, storedHooks, logger)
		return
	}
	// Orphan is written while the slot is still busy. Reset leaves a busy slot to the goroutine
	// that owns the transition, so nothing else can write this incarnation's dispose line first.
	logger.Debug(MsgOrphan, "key", key)

	if storedHooks.EnforceCloseBeforeOpen && grace <= 0 {
		// Zero grace keeps nothing: Close while still slotBusy, then unmap so a racing Open
		// waits instead of creating during Close.
		t.endMappedClose(key, incarnation, storedHooks, logger)
		return
	}

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
// it. When the stored EnforceCloseBeforeOpen is set, Close runs as slotBusy so a racing Open
// waits; otherwise the key is unmapped first (dest) so a concurrent Open may create during Close.
func (t *Table) expire(key string, incarnation *slot) {
	t.mu.Lock()
	if incarnation.state != slotAsleep || incarnation.holders > 0 {
		t.mu.Unlock()
		return
	}
	storedHooks := incarnation.hooks
	logger := incarnation.logger
	if storedHooks.EnforceCloseBeforeOpen {
		incarnation.state = slotBusy
		incarnation.ready = make(chan struct{})
		t.mu.Unlock()
		t.endMappedClose(key, incarnation, storedHooks, logger)
		return
	}
	incarnation.state = slotGone
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	t.mu.Unlock()
	dispose(key, storedHooks, logger)
}

// Reset ends every incarnation on this table. An awake value is slept first, so Close never sees
// a live value and orphan still precedes dispose when Sleep returns. A Sleep panic skips orphan
// and still disposes. Tests only: it must not race an Open on the same key. A slot that is
// mid-transition is ended by the goroutine that owns that transition. Reset unmaps first
// regardless of EnforceCloseBeforeOpen.
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
			if recovered := runHook(storedHooks.Sleep); recovered != nil {
				logger.Error(MsgHookPanic, "key", key, "hook", "sleep", "panic", recovered)
			} else {
				logger.Debug(MsgOrphan, "key", key)
			}
			dispose(key, storedHooks, logger)
		case slotAsleep:
			dispose(key, storedHooks, logger)
		case slotBusy, slotGone:
			// The goroutine that owns this transition ends the incarnation itself: it finds the
			// slot unmapped, or its grace wait already released by the loop above.
		}
	}
}
