# Zero-grace Table.Open can return a pointer whose Close already ran

Bug: zero-grace Table.Open can return a value whose Close hook has already run when the holder ctx is canceled before Open returns (reproduced: cancel during blocking create).

Agreed how (implement this, not a different design):
`(value, nil)` means this call bound a holder that was still live at return. If `ctx.Err() != nil` at bind time, Open returns `(nil, ctx.Err())` and does not give the caller the pointer.
After a successful create, awake bind, or reclaim: if `ctx.Err() != nil`, call `drop` on this stack (do not register AfterFunc) and return the context error. The value is not leaked: drop still sleeps/graces/closes it; waiters that bound when ready closed still keep it alive. If ctx is still live, keep AfterFunc/watch.
Do not only delay AfterFunc until after return. Do not treat a pre-create ctx.Err() check as the only fix; the reproduced case cancels during create. That check may be added as an extra, not instead.
Same three sites: put, awake bind in Open, reclaimLocked.

Tests first, then fix
1. Land product tests that FAIL on current master (reproduce). Put them in reclaim/ so `go test ./reclaim` fails until the fix.
2. Then implement the agreed how.
3. Confirm those tests PASS and existing reclaim tests stay green.
Do not weaken the tests to pass. Do not fix the other two reclaim bugs (hook panic bricks key; unmap-before-Close overlap).

Example reproducer (adapt into package reclaim tests; fail while Close already ran and Open returned the pointer):
```go
func TestOpenCanceledDuringCreateZeroGraceDisposedIncarnation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	for i := 0; i < 50; i++ {
		tab := NewTable(0)
		ctx, cancel := context.WithCancel(context.Background())
		item := &struct{ closed atomic.Bool }{}
		var closeRan atomic.Bool
		started := make(chan struct{})
		go func() { <-started; cancel() }()
		value, err := tab.Open(ctx, "k", logger, func() (any, error) {
			close(started)
			<-ctx.Done()
			return item, nil
		}, Hooks{Close: func() { closeRan.Store(true) }})
		if err != nil {
			t.Fatalf("iter %d: Open: %v", i, err)
		}
		if closeRan.Load() {
			t.Fatalf("iter %d: Close ran before Open returned; ptr=%p", i, value)
		}
	}
}
```
After the fix this test must be updated so the contract is: Open returns ctx.Err() and not the pointer; Close may run because the holder was already done. Also cover awake bind and reclaimLocked with already-done ctx (same contract). Cover that waiters still get the live value.
