## 1. Tests first (must FAIL on dest)

- [x] 1.1 Add `windowcounter/repro_buffered_outage_test.go` copied from the parent repro; call dest `Kill` (do not port `trackConn` / `closeListenerAndConns`); port only helpers this ticket needs
- [x] 1.2 Rewrite assertions to `err=nil`, admit until this node's `limit`, then `allowed=false` (they currently fail wanting `redis:unreachable`)
- [x] 1.3 Rewrite dest fail-closed cases to that same lock: `TestTake_BufferedPendingDeltaOutage`, `TestTake_BufferedFlushThenKillFailsClosed`, `TestTake_BufferedTwoInstancesOutage` (each instance's own `limit`, nil error), `TestTake_BufferedSleepStoresFlushError`, `TestPeek_BufferedEmptyFlushThenKill`. Keep `TestTake_Unreachable` / `TestPeek_Unreachable`
- [x] 1.4 Run `go test -short -count=1 -timeout 60s -run 'TestRepro_BufferedTakeHidesRedisOutage|TestTake_BufferedPendingDeltaOutage|TestTake_BufferedFlushThenKillFailsClosed|TestTake_BufferedTwoInstancesOutage|TestTake_BufferedSleepStoresFlushError|TestPeek_BufferedEmptyFlushThenKill' ./windowcounter` and confirm FAIL on dest's fail-closed lock

## 2. Buffered outage is local cap (do not fail-closed)

- [x] 2.1 On buffered GET error, use existing `redisKnown + localDelta` (seed 0 on first sight) and return `err=nil`; store the outage on `lastFlushErr` so later Take/Peek skip share-refresh GET
- [x] 2.2 Stop returning `lastFlushErr` from buffered Take/Peek; remove `bufferedOutageErrorLocked` as an error-returning helper; keep storing a failed flush in `flushPendingLocked`. Do not change `takeBuffered` to fail-closed. Do not GET or INCR every buffered Take to probe. Do not return `redis:unreachable` because `localDelta > 0` skipped GET
- [x] 2.3 Update `New` comment, `README.md`, and `knowledge/devdocs/std_go_windowcounter.md` (`sync_rate` Language + How to use + Gotcha): buffered outage is per-node `limit` with nil error; exact mode still returns Redis errors. Do not tell operators to check `err` to fail closed on this path

## 3. Confirm PASS

- [x] 3.1 Re-run the new lock tests and confirm PASS
- [x] 3.2 Run `go test -short -count=1 -timeout 60s ./windowcounter` and confirm PASS (exact-mode unreachable tests still error)
- [x] 3.3 Run `openspec validate windowcounter-buffered-outage-local-cap --type change --strict`
