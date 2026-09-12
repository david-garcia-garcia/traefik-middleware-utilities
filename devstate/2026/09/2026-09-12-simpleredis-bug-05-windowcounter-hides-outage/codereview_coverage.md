# Test coverage

1. [hard] Critical path untested — `windowcounter/limiter.go:417` — `bufferedOutageErrorLocked` returns stored `lastFlushErr` even while `lastRedisOK` is still inside `sync_rate`; `flushPendingLocked` is what Sleep/Close/flushLoop use to store it. Ticket job (buffered Take/Peek must not return a silent nil after a failed flush) is proven only via the k=1 clock inject: `TestTake_BufferedPendingDeltaOutage` does `now.Add(time.Hour)` then Take, which hits `flushPendingLocked()` at `:425`, not the retained-error arm. Revert `if l.lastFlushErr != nil { return l.lastFlushErr }` and that test stays green. No test: Kill, Sleep/Close with a pending delta, Take without advancing `now`.
   → After Kill, `Sleep` (long `syncRate`, clock frozen), then Take/Peek and assert a Redis outage error
   Status: done
   Argument: TestTake_BufferedSleepStoresFlushError — Kill, Sleep, Take/Peek with frozen clock.
2. [hard] Edge case untested — `windowcounter/limiter.go:435` — empty-flush Peek probe (`getCount` when `localDelta == 0` so EVAL is a no-op). Spec/design: Peek uses the same staleness path; after one missed `sync_rate` a nil Peek is a silent fallback. `TestTake_BufferedFlushThenKillFailsClosed` only Takes (`windowLocked` GET at `:242`); `TestTake_BufferedPendingDeltaOutage` Peeks after Take already set `lastFlushErr`. `(none)` hits the GET-one-key loop.
   → Flush until `localDelta == 0`, Kill, `SetNowForTest` + one `sync_rate`, Peek, assert a Redis outage error
   Status: done
   Argument: TestPeek_BufferedEmptyFlushThenKill — Sleep while up, Kill, advance one sync_rate, Peek.
