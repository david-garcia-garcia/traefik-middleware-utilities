# Spec

1. [missing] Saturated wait under Yaegi: short-context callers — `simpleredis/repro_yaegi_select_probe_test.go:196` — the impatient 1 ms callers in `SelectProbeBorrowWait` count only `nilNil` and never check that `err` is their context error, so the scenario bullet "each short-context caller returns its context error" (tcp-session spec, Requirement "Interpreted concurrency does not cross turns or deadlines") has no interpreted or compiled proof
   Quote:
   ```
   - **AND** each short-context caller returns its context error
   ```
   Fix: In the impatient loop, count calls whose `err` is not `context.DeadlineExceeded`/`context.Canceled` (e.g. `impatientNotCtx`) and add that key to `borrowWaitAnomalies`
   Status: done
   Argument: SelectProbeBorrowWait counts impatientNotCtxErr (a socket, or an error other than context.DeadlineExceeded) and borrowWaitAnomalies fails on it; runs compiled and under Yaegi (5/5 green, 5345 impatient calls per round, 0 anomalies).
2. [judgement] Interpreted timing bounds looser than the scenarios — `simpleredis/repro_yaegi_select_probe_test.go:156` — the patient check fails only below 700 ms against an 800 ms `PoolTimeout` (scenario says "no earlier than its own `PoolTimeout`"), and the long backoff check at line 80 fails only below 550 ms for a 600 ms delay (scenario says "no 600 millisecond backoff returns before its own delay"); design D6 sets these tolerances, and the shipped code (`time.AfterFunc(PoolTimeout)`, `context.WithTimeout(ctx, delay)`) cannot fire early, so the behaviour is met but the Yaegi proof is weaker than the spec text
   Note:
   ```
   The compiled TestTurnWaitPoolTimeoutIsPoolWait asserts waited >= PoolTimeout exactly; no compiled or interpreted test asserts the 600 ms backoff >= its delay.
   ```
   Fix: Either tighten the bounds to `< PoolTimeout` / `< 600 ms` (timers do not fire early), or reword the two scenario bullets to state the tolerance design D6 chose
   Status: skipped
   Argument: Judgement: design D6 chose the 700/550 ms margins to absorb interpreter scheduling on loaded CI runners; the shipped timers cannot fire early and compiled TestTurnWaitPoolTimeoutIsPoolWait asserts waited >= PoolTimeout exactly.
