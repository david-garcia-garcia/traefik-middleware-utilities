# Test coverage

1. [hard] Edge case untested — `simpleredis/commands_exec.go:147` — rewritten `waitUntil` has an explicit ctx-fired arm (cancel or deadline during retry backoff returns `ctx.Err()`; design D4 rejected `time.Sleep` for exactly this), but no compiled or Yaegi test fires the ctx before the delay ends; the `WaitUntil` probe only uses contexts that outlive the delay
   Fix: Add a compiled test plus a `SelectProbeWaitUntil` arm that cancels a ctx a few ms into a long `waitUntil` delay and asserts `context.Canceled` (and a short-deadline case asserts `context.DeadlineExceeded`) well before the delay
   Status: done
   Argument: SelectProbeWaitUntil gained cancelled (20 ms) and deadline (30 ms) waiters on a 2 s delay that must return context.Canceled / context.DeadlineExceeded within 400 ms; waitUntilAnomalies fails on cancelNotCanceled / deadlineNotExceeded. Runs in the _Compiled control (including -race) and in _Yaegi.
   Quote:
   ```
   waitCtx, cancel := context.WithTimeout(ctx, delay)
   <-waitCtx.Done()
   cancel()
   if err := ctx.Err(); err != nil {
       return err
   }
   // tests grepped: waitUntil in *_test.go -> only repro_yaegi_select_probe_test.go
   // (long: 5s ctx / 600ms delay; short: 300ms ctx / 1ms delay) -> ctx never fires first: (none)
   ```
2. [judgement] Assertion does not prove the job — `simpleredis/repro_yaegi_select_probe_test.go:285` — `SelectProbeSaturatedGet` is the only Yaegi probe that can reach the FIFO hand-off (`BorrowWait` holds the only turn throughout, `TwoPools` never saturates), but it never requires a Get to succeed or a waiter to be granted; a run where every call ends in pool wait or ctx error passes, so an interpreted hand-off that never fires stays green
   Fix: Count successful Gets (value `t`, nil error) and fail `saturatedGetAnomalies` when that count is 0, as `TestTurnGrantAbandonRaceKeepsSemaphoreBalanced` does with `granted`
   Status: skipped
   Argument: Judgement: a success floor in the saturated Yaegi probe would tie it to runner load; the FIFO hand-off is proven by TestTurnHandoffIsFIFO and TestTurnGrantAbandonRaceKeepsSemaphoreBalanced (granted > 0), and the interpreted grant runs the same freeInUseTurn source.
3. [judgement] Assertion does not prove the job — `simpleredis/repro_yaegi_select_probe_test.go:196` — the 16 impatient 1 ms callers in `SelectProbeBorrowWait` only feed `nilNil`; the spec scenario requires each to return its context error, so a crossed wake that returns `ErrPoolWait` or a socket under Yaegi is not detected
   Fix: Count impatient calls whose error is not `context.DeadlineExceeded` (or that return a socket) and add that counter to `borrowWaitAnomalies`
   Status: done
   Argument: Same change as Spec 1: impatientNotCtxErr counter in SelectProbeBorrowWait, checked by borrowWaitAnomalies.
4. [judgement] Assertion does not prove the job — `simpleredis/pool_test.go:849` — `TestTurnGrantAbandonRaceKeepsSemaphoreBalanced` counts any error as `abandoned`; it does not check that the error is `ErrPoolWait` or the caller's ctx error, which the racing-handoff scenario names
   Fix: Fail on an error that is neither `IsPoolWait(err)` nor `errors.Is(err, context.Canceled/DeadlineExceeded)`
   Status: skipped
   Argument: Judgement: a waiter granted after its ctx fired continues into dial, which can fail with an I/O-class error, so restricting errors to pool wait or ctx would make the race test flaky; its job is the at-rest balance it asserts.
5. [judgement] Edge case untested — `simpleredis/pool.go:186` — after a grant the waiter stops its PoolTimeout `time.AfterFunc` and its `context.AfterFunc` (spec: the wait MUST NOT leak its timeout or cancel wake-up), but deleting both calls leaves every test green because a late `abandon` finds the waiter gone and no-ops
   Fix: Assert that `poolTimer.Stop` / the ctx-abandon stop report a pending registration after a grant (e.g. a granted waiter on a long-lived cancelable ctx, then check the AfterFunc stop returned true), or record as accepted if not worth a seam
   Status: skipped
   Argument: Judgement: a late abandon is a locked no-op by design, and asserting the Stop results needs a seam in the wait path; an unstopped timer would fire once after PoolTimeout and the ctx AfterFunc is released when ctx ends.
