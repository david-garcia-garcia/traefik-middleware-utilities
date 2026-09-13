# Performance

1. [hard] Unbounded collection — `windowcounter/limiter.go:304` — `seedEmptyWindowLocked` writes `l.windows[redisKey]` on first sight when GET fails or `lastFlushErr` is set; dest returned the GET error and did not insert. `takeBuffered` then does `localDelta++`, so `flushPendingLocked` skips `delete` until a flush succeeds (`localDelta == 0`). Grows with unique opaque keys × a new `:{windowStart}` slot each window for the whole outage; no max size. Healthy-path expireAt eviction does not run on these entries. Each flush tick also `Eval`s every retained pending key while holding `l.mu`.
   → Cap `l.windows` with eviction (reuse expireAt delete even while a flush is failing, or a max entry count)
   Status: skipped
   Argument: Bound the ask — eviction/max size is extra vs Desired; noted `knowledge/debt/2026-09-13-windowcounter-outage-windows-unbounded.md`.
