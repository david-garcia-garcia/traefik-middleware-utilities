package simpleredis

// Probes for the Yaegi v0.16.1 shared-select bug. interp/run.go _select allocates one
// []reflect.SelectCase per select statement and every execution overwrites it without a lock
// before reflect.Select. Two goroutines on the same statement with different channels can
// therefore wait on, or receive from, each other's channels. Each probe drives one site that
// used a select (waitUntil, the borrowSocket wait, the in-use-turn take and give-back) from many
// goroutines and reports every way a crossed wait shows up.
//
// This file has no testing import on purpose: repro_yaegi_select_crosstalk_test.go runs these
// probes compiled (the control, which must report no anomaly) and copies this file into the
// interpreted simpleredis package to run them under Yaegi.
//
// The probes do not use select themselves: the watchdog is a buffered channel that either the
// WaitGroup or a time.AfterFunc fills, read with a plain receive. Otherwise the interpreted probe
// would depend on the primitive it is measuring.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// selectProbeWatchdog is how long a probe waits for its callers before it counts the rest as hung.
const selectProbeWatchdog = 5 * time.Second

// inUseTurnsForTest reports the free in-use turns and the frozen turn cap. The interpreted package
// only sees what the harness copies, so the probes and the compiled tests share this helper.
func inUseTurnsForTest(sr *SimpleRedis) (free, capacity int) {
	sr.inUseMu.Lock()
	free, capacity = sr.freeTurns, sr.turnCap
	sr.inUseMu.Unlock()
	return free, capacity
}

// holdTurnForTest takes one free in-use turn without a socket and reports whether one was free.
// Return it with sr.freeInUseTurn.
func holdTurnForTest(sr *SimpleRedis) bool {
	sr.inUseMu.Lock()
	took := sr.freeTurns > 0
	if took {
		sr.freeTurns--
	}
	sr.inUseMu.Unlock()
	return took
}

// waitAllOrWatchdog blocks until wg is done or selectProbeWatchdog elapses, without a select.
func waitAllOrWatchdog(wg *sync.WaitGroup) {
	done := make(chan struct{}, 2)
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	watchdog := time.AfterFunc(selectProbeWatchdog, func() { done <- struct{}{} })
	<-done
	watchdog.Stop()
}

// SelectProbeWaitUntil runs waitUntil (commands_exec.go) from many goroutines at once.
// Long waiters ask for 600ms on a ctx that stays live; each must sleep at least that long.
// Short waiters ask for 1ms on a 300ms ctx in a loop; each must return nil, quickly.
// longEarly: a long waiter woke on another goroutine's timer or canceled ctx.
// shortErr / shortSlow: a short waiter lost its own tick and sat until its ctx deadline.
func SelectProbeWaitUntil() map[string]int64 {
	var longCalls, longEarly, shortCalls, shortErr, shortSlow atomic.Int64
	stop := time.Now().Add(400 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			start := time.Now()
			_ = waitUntil(ctx, 600*time.Millisecond)
			longCalls.Add(1)
			if time.Since(start) < 550*time.Millisecond {
				longEarly.Add(1)
			}
		}()
	}
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				start := time.Now()
				err := waitUntil(ctx, time.Millisecond)
				cancel()
				shortCalls.Add(1)
				if err != nil {
					shortErr.Add(1)
				}
				if time.Since(start) > 100*time.Millisecond {
					shortSlow.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	return map[string]int64{
		"longCalls":  longCalls.Load(),
		"longEarly":  longEarly.Load(),
		"shortCalls": shortCalls.Load(),
		"shortErr":   shortErr.Load(),
		"shortSlow":  shortSlow.Load(),
	}
}

// SelectProbeBorrowWait runs the blocking borrowSocket wait (pool.go) with the only turn held,
// so every call waits for a turn, its own ctx and its own PoolTimeout.
// Patient callers have a live 5s ctx and an 800ms PoolTimeout: each must get errPoolWait after ~800ms.
// Cancelled callers have a live ctx that is cancelled 50ms in: each must get context.Canceled well
// before PoolTimeout.
// Impatient callers loop with a 1ms ctx and must get a ctx error.
// nilNil: borrowSocket returned no socket and no error (exec would call runOnConn with a nil conn).
// patientEarly: a patient caller woke on another caller's ctx or timer.
// cancelLate: a cancelled caller did not wake on its own cancel, or returned something else.
// hung: calls still blocked 4s after the last deadline (for example in <-timer.C after a stolen tick).
func SelectProbeBorrowWait(host string) map[string]int64 {
	sr, err := New(Config{Host: host, PoolSize: 1, MaxIdleConns: 1, PoolTimeout: 800 * time.Millisecond, CommandTimeout: 5 * time.Second})
	if err != nil {
		return map[string]int64{"newErr": 1}
	}
	if !holdTurnForTest(sr) { // hold the only turn for the whole probe
		return map[string]int64{"newErr": 1}
	}

	var started, finished, nilNil, patientCalls, patientEarly, patientNotPoolWait, impatientCalls atomic.Int64
	var cancelCalls, cancelLate atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		started.Add(1)
		go func() {
			defer wg.Done()
			defer finished.Add(1)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			start := time.Now()
			conn, err, _, _ := sr.borrowSocket(ctx, false)
			patientCalls.Add(1)
			if conn == nil && err == nil {
				nilNil.Add(1)
			}
			if conn != nil {
				sr.release(conn, false)
			}
			if !errors.Is(err, errPoolWait) {
				patientNotPoolWait.Add(1)
			}
			if time.Since(start) < 700*time.Millisecond {
				patientEarly.Add(1)
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		started.Add(1)
		go func() {
			defer wg.Done()
			defer finished.Add(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			canceler := time.AfterFunc(50*time.Millisecond, cancel)
			defer canceler.Stop()
			start := time.Now()
			conn, err, _, _ := sr.borrowSocket(ctx, false)
			cancelCalls.Add(1)
			if conn == nil && err == nil {
				nilNil.Add(1)
			}
			if conn != nil {
				sr.release(conn, false)
			}
			if !errors.Is(err, context.Canceled) || time.Since(start) > 400*time.Millisecond {
				cancelLate.Add(1)
			}
		}()
	}
	stop := time.Now().Add(500 * time.Millisecond)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		started.Add(1)
		go func() {
			defer wg.Done()
			defer finished.Add(1)
			for time.Now().Before(stop) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				conn, err, _, _ := sr.borrowSocket(ctx, false)
				cancel()
				impatientCalls.Add(1)
				if conn == nil && err == nil {
					nilNil.Add(1)
				}
				if conn != nil {
					sr.release(conn, false)
				}
			}
		}()
	}

	waitAllOrWatchdog(&wg)
	hung := started.Load() - finished.Load()
	if hung == 0 {
		sr.freeInUseTurn()
		sr.Close()
	}
	return map[string]int64{
		"nilNil":             nilNil.Load(),
		"patientCalls":       patientCalls.Load(),
		"patientEarly":       patientEarly.Load(),
		"patientNotPoolWait": patientNotPoolWait.Load(),
		"impatientCalls":     impatientCalls.Load(),
		"cancelCalls":        cancelCalls.Load(),
		"cancelLate":         cancelLate.Load(),
		"hung":               hung,
	}
}

// SelectProbeTwoPools drives Get on two clients at once, so the uncontended turn take and the
// give-back run concurrently on two different semaphores. After all callers return, each
// semaphore must be full again and no over-free may have been counted. PoolSize is above the
// caller count so no call reaches the blocking wait: that is SelectProbeBorrowWait's and
// SelectProbeSaturatedGet's.
func SelectProbeTwoPools(host string) map[string]int64 {
	cfg := Config{Host: host, PoolSize: 32, MaxIdleConns: 32, PoolTimeout: 50 * time.Millisecond, CommandTimeout: time.Second, MaxRetries: -1}
	a, err := New(cfg)
	if err != nil {
		return map[string]int64{"newErr": 1}
	}
	defer a.Close()
	b, err := New(cfg)
	if err != nil {
		return map[string]int64{"newErr": 1}
	}
	defer b.Close()

	var calls, errs atomic.Int64
	stop := time.Now().Add(400 * time.Millisecond)
	var wg sync.WaitGroup
	for _, client := range []*SimpleRedis{a, b} {
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(sr *SimpleRedis) {
				defer wg.Done()
				for time.Now().Before(stop) {
					_, err := sr.Get(context.Background(), "hit")
					calls.Add(1)
					if err != nil {
						errs.Add(1)
					}
				}
			}(client)
		}
	}
	wg.Wait()
	freeA, capA := inUseTurnsForTest(a)
	freeB, capB := inUseTurnsForTest(b)
	return map[string]int64{
		"calls":      calls.Load(),
		"errs":       errs.Load(),
		"capA":       int64(capA),
		"freeA":      int64(freeA),
		"overFreesA": a.overFrees.Load(),
		"capB":       int64(capB),
		"freeB":      int64(freeB),
		"overFreesB": b.overFrees.Load(),
	}
}

// SelectProbeSaturatedGet drives public Get from 24 goroutines on a client with PoolSize 2, a 20ms
// PoolTimeout and caller deadlines of 1-5ms, so turns are granted to waiters, abandoned on ctx and
// timed out on PoolTimeout at the same time. This is the path that panicked the process under Yaegi:
// a (nil, nil) borrow reaches runOnConn with a nil socket.
// panics: a Get panicked (recovered per call).
// wrongValue: Get returned no error and not the stored value.
// unexpectedErr: an error that is neither pool wait, a ctx error, nor a classified unreachable.
// free / cap / overFrees: the semaphore at rest after every caller returned.
// hung: callers still running when the watchdog fired.
func SelectProbeSaturatedGet(host string) map[string]int64 {
	sr, err := New(Config{Host: host, PoolSize: 2, MaxIdleConns: 2, PoolTimeout: 20 * time.Millisecond, CommandTimeout: time.Second})
	if err != nil {
		return map[string]int64{"newErr": 1}
	}

	var started, finished, calls, panics, wrongValue, unexpectedErr atomic.Int64
	stop := time.Now().Add(400 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		started.Add(1)
		go func(worker int) {
			defer wg.Done()
			defer finished.Add(1)
			for n := 0; time.Now().Before(stop); n++ {
				timeout := time.Duration(1+(worker+n)%5) * time.Millisecond
				func() {
					defer func() {
						if recover() != nil {
							panics.Add(1)
						}
					}()
					ctx, cancel := context.WithTimeout(context.Background(), timeout)
					defer cancel()
					value, err := sr.Get(ctx, "hit")
					calls.Add(1)
					// Plain ifs, not a tagless switch with a multi-expression case: Yaegi v0.16.1 evaluated
					// that case list wrongly and reported classified errors as unexpected.
					expected := IsUnreachable(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
					if err == nil && string(value) != "t" {
						wrongValue.Add(1)
					}
					if err != nil && !expected {
						unexpectedErr.Add(1)
					}
				}()
			}
		}(i)
	}
	waitAllOrWatchdog(&wg)
	hung := started.Load() - finished.Load()
	free, capacity := inUseTurnsForTest(sr)
	if hung == 0 {
		sr.Close()
	}
	return map[string]int64{
		"calls":         calls.Load(),
		"panics":        panics.Load(),
		"wrongValue":    wrongValue.Load(),
		"unexpectedErr": unexpectedErr.Load(),
		"free":          int64(free),
		"cap":           int64(capacity),
		"overFrees":     sr.overFrees.Load(),
		"hung":          hung,
	}
}
