# Test coverage

1. [hard] Critical path untested — `reclaim/table.go:148` — `dispose` recovers a Close panic so AfterFunc cannot kill the process; no test panics Close (`TestTable_SleepPanicAfterFuncDoesNotCrash` Close only increments a counter)
   → Assert a Close panic on the AfterFunc path leaves the process alive, ready already closed, and a later Open does not hang
   Status: done
   Argument: added `TestTable_ClosePanicAfterFuncDoesNotCrash`.
2. [judgement] Happy path only — `reclaim/table.go:289` — Wake panic sets `createErr` so waiters replay the wrapped error; `TestTable_WakePanicReturnsErrorAndUnsticks` only asserts the panicking Open and a later sequential Open
   → Assert a concurrent Open waiting on that Wake receives the same wrapped error, not a new create
   Status: skipped
   Argument: judgement; sequential later Open already proves unstick; concurrent waiter is the same createErr path as create error.
3. [judgement] Happy path only — `reclaim/table.go:437` — `Reset` recovers Sleep panic and skips `reclaim_orphan`; no test panics Sleep (or Close) under Reset
   → Assert Reset with a panicking Sleep still disposes without crashing, or skip because Reset is tests-only and `drop` already covers Sleep panic
   Status: skipped
   Argument: judgement; Reset is tests-only; drop AfterFunc already covers Sleep panic.
