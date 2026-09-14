package reclaim

import (
	"runtime"
	"strconv"
	"testing"
	"time"
)

// leakBudget bounds the wait for watch goroutines to exit. It is not a timing assertion: the
// incarnations have already ended when it starts, so nothing is left for those goroutines to do.
const leakBudget = 2 * time.Second

// TestRepro_NilDoneHolderLeaksWatchGoroutine checks whether a holder whose Done() is nil and whose
// Err() is never set leaves its watcher behind. dropWhenDone starts go t.watch for that shape, and
// waitCtx polls Err every 20ms with no exit other than cancellation, so the goroutine outlives the
// incarnation it was watching and keeps that key and slot reachable for the life of the process.
func TestRepro_NilDoneHolderLeaksWatchGoroutine(t *testing.T) {
	const keys = 20
	const slack = 2
	base := settledGoroutines()

	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	for i := 0; i < keys; i++ {
		if _, err := tab.Open(newNeverCanceledNilDone(), "k"+strconv.Itoa(i), recLogger(h), func() (any, error) {
			return &lifecycle{}, nil
		}, Hooks{}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}

	// Reset ends every incarnation, so no watcher has a slot left to drop a holder from.
	tab.Reset()
	waitUntil(t, func() bool { return countMsg(h.events(), MsgDispose) == keys })

	deadline := time.Now().Add(leakBudget)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+slack {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%d goroutines after every incarnation ended, baseline %d and slack %d: %d watchers are still polling",
		runtime.NumGoroutine(), base, slack, keys)
}

// newNeverCanceledNilDone is the holder shape this repro needs: Done() is nil, so the table polls,
// and Err() never reports cancellation. context.Background() behaves this way.
func newNeverCanceledNilDone() *nilDoneCtx { return &nilDoneCtx{} }
