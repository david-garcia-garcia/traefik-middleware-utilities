# Requirement
IssueKey: 2026-09-13-windowcounter-bug-buffered-outage

## Problem
Buffered Take when Redis is down is accepted per-node fallback: keep this instance's `limit`, `err=nil`. It is not a fail-closed product defect. Dest already returns a Redis error on buffered Take after a missed `sync_rate` or a failed flush (PR #30). Spec “Redis errors propagate” on buffered Take disagrees with that accepted contract.

## Current (code)
- `windowcounter/limiter.go` `takeBuffered` — admits from `redisKnown + localDelta` then returns `bufferedOutageErrorLocked` as `err`.
- `windowcounter/limiter.go` `bufferedOutageErrorLocked` — returns stored `lastFlushErr`, else probes via `flushPendingLocked` / GET after one missed `sync_rate`. That is a Redis contact on Take, not a silent local cap.
- `windowcounter/limiter.go` `windowLocked` — GET when `localDelta == 0`; skip GET when `localDelta > 0`.
- `windowcounter/limiter.go` `takeExact` — `Incr` / `Expire` / GET errors return on that call.
- `windowcounter/limiter.go` `New` comment — buffered mode “returns a retained flush error (or a probe after one missed sync_rate) instead of a silent nil”.
- `windowcounter/limiter.go` `flushPendingLocked` — stores `lastFlushErr` on EVAL failure; Sleep/Close/flushLoop call this.
- `windowcounter/limiter_test.go` `TestTake_BufferedPendingDeltaOutage` / `TestTake_BufferedFlushThenKillFailsClosed` / `TestTake_BufferedTwoInstancesOutage` / `TestTake_BufferedSleepStoresFlushError` — `fake.Kill()` then `wantRedisOutage`.
- `windowcounter/fake_redis_test.go` `Kill` — closes listener and accepted sockets (same job as parent `closeListenerAndConns`). Dest has no `repro_buffered_outage_test.go`.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — “Redis errors propagate” includes buffered pending-delta; nil error with a local admit MUST NOT occur.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — “Failed buffered flush is retained”; Take/Peek SHALL return the stored error.
- `knowledge/devdocs/std_go_windowcounter.md` and `README.md` — buffered Take/Peek return the retained flush error; check `err` to fail closed.
- Archived `openspec/changes/archive/2026-09-12-windowcounter-buffered-flush-error/` — dest landed fail-closed for this path.

## Desired
1. Tests first. Copy/adapt caller `windowcounter/repro_buffered_outage_test.go` onto dest. Rewrite assertions from `redis:unreachable` to the agreed lock: `err=nil`, admit until this node's `limit`, then local deny. They currently fail wanting unreachable.
2. Port only the fake helpers this ticket needs. Dest `Kill` already closes listener and sockets; do not invent a second kill API unless the lock test needs the parent names.
3. Buffered Take while Redis is down: per-node `limit`, `err=nil`. Do not GET or INCR every buffered Take to probe Redis. Do not return `redis:unreachable` solely because `localDelta > 0` skipped GET.
4. Do not change `takeBuffered` to fail-closed (do not keep or add returning a Redis error as the buffered-outage contract).
5. Exact mode still returns Redis errors.
6. Spec “Redis errors propagate” on buffered Take is a deviation: rewrite that requirement (and lock tests) for nil error + local deny after `limit`. Document the deviation (devdocs + spec delta).
7. Then `go test -short -count=1 -timeout 60s ./windowcounter` (and the new tests) PASS.

## Affected
- `windowcounter/` tests (new lock test; rewrite dest fail-closed cases that contradict the ticket)
- `windowcounter/limiter.go` (`takeBuffered` error return / `bufferedOutageErrorLocked` — only as needed so buffered Take is not fail-closed)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md`
- `knowledge/devdocs/std_go_windowcounter.md`
- `README.md` (buffered outage wording)

## Out of scope
- Bugs 2–7 (stale previous, Sleep/Wake hang, exact EXPIRE retry, lock during GET, fractional window, Peek occupancy)
- GET or INCR on every buffered Take to probe Redis
- Returning `redis:unreachable` because `localDelta > 0` skipped GET
- Changing exact-mode error propagation

## Unknowns
- Peek uses the same `bufferedOutageErrorLocked` as Take. Ticket names buffered Take. Whether Peek stays on the dest fail-closed probe or follows Take's nil-error fallback is not stated.
- Whether dest `Kill` is enough for the lock test versus porting parent `trackConn` / `closeListenerAndConns` names.
- Whether `lastFlushErr` / probe helpers are removed or only disconnected from buffered Take.

## Tensions
- Ticket: accepted per-node fallback, `err=nil`. Dest and live specs (PR #30 / `std_go_windowcounter_sliding-take` / `std_go_windowcounter_sync-flush`) require a Redis error after flush failure or one missed `sync_rate`. Ticket wins; specs and tests are the rewrite, not a second product ask.
- Caller repro asserts `redis:unreachable`. After the lock rewrite it must assert nil error and local deny after `limit`. Do not keep the fail-closed assertion as the post-fix contract.
- Ticket: do not change `takeBuffered` to fail-closed. Dest `takeBuffered` already returns `bufferedOutageErrorLocked`. Honouring the ticket means that error is no longer the buffered-outage result, not adding a new fail-closed branch.
- README/devdocs tell operators to check `err` to fail closed. Ticket says document the deviation (per-node cap, nil error).
