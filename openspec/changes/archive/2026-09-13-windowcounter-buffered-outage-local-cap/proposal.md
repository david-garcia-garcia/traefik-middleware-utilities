## Why

Dest buffered `Take`/`Peek` (`sync_rate > 0`) still returns a Redis error after a missed `sync_rate` or a failed flush (PR #30). The accepted contract is the opposite: keep this node's `limit`, `err=nil`. Dest specs and fail-closed tests are the rewrite, not a second product ask.

## What Changes

- **BREAKING** for callers that treat a buffered-outage Redis error as fail-closed: buffered Take and Peek return the local admit/deny and `err=nil` while Redis is down. Admit until this node's `limit`, then local deny. Combined admits across instances MAY exceed global `limit`.
- Exact mode (`sync_rate == 0`) still returns Redis errors (`redis:unreachable` / `redis:timeout`).
- Do not GET or INCR every buffered Take to probe Redis. Do not return `redis:unreachable` because `localDelta > 0` skipped GET. Do not keep or add returning a Redis error as the buffered-outage result.
- Tests first: copy/adapt parent `repro_buffered_outage_test.go`, rewrite assertions to `err=nil` + local deny after `limit` (they currently fail wanting unreachable). Call dest `Kill`. Rewrite dest fail-closed cases that contradict that lock.
- Rewrite `std_go_windowcounter_sliding-take` “Redis errors propagate” (buffered pending-delta) and `std_go_windowcounter_sync-flush` retained-flush / stale-probe / two-instance requirements. Document the deviation in those deltas plus usage/README after the code is true.

## Capabilities

### New Capabilities

- None. Fold into the existing windowcounter leaves.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: Redis errors still propagate in exact mode. Buffered Take/Peek while Redis is down SHALL return `err=nil` and the local admit/deny up to this node's `limit`. A nil error with a local admit MUST occur; `redis:unreachable` solely because `localDelta > 0` skipped GET MUST NOT.
- `std_go_windowcounter_sync-flush`: Failed flush is still stored so later buffered Take/Peek skip Redis contact. Take/Peek MUST NOT return that error and MUST NOT probe after one missed `sync_rate`. Two instances MAY each admit their own `limit`. Construction/README/devdocs state per-node cap with nil error, not fail-closed.

## Impact

- `windowcounter/limiter.go` (`takeBuffered` / `peekBuffered` / `windowLocked` GET-error path / remove `bufferedOutageErrorLocked` as an error-returning helper). Do not change `takeBuffered` to fail-closed.
- `windowcounter/` tests: new lock test; rewrite dest fail-closed cases (`TestTake_BufferedPendingDeltaOutage` and siblings). Dest `Kill` is enough.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` and `std_go_windowcounter_sync-flush/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md`, `README.md` (`New` comment) after the code is true.
- No GET/INCR-on-every-Take probe. No exact-mode change. Bugs 2–7 out of scope. Caller still owns the opaque key.
