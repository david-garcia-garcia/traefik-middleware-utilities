# Requirement
IssueKey: 2026-09-13-windowcounter-bug-lock-during-get

## Problem
Buffered Take/Peek and the flush ticker hold the limiter mutex across Redis GET and Eval. A delayed GET on one opaque key blocks Take on an unrelated key for the GET duration (~400ms in the repro). Fast Take must finish well under that delay.

## Current (code)
- `windowcounter/limiter.go` `takeBuffered` — `l.mu.Lock()` then `defer Unlock()`; `windowLocked` and `bufferedCountLocked` run Redis GET before Unlock.
- `windowcounter/limiter.go` `peekBuffered` — same lock; `peekCountLocked` and `bufferedCountLocked` GET before Unlock.
- `windowcounter/limiter.go` `windowLocked` — GET on first sight of the key, and again when `localDelta == 0` on an already-mapped current key. Called while `l.mu` is held.
- `windowcounter/limiter.go` `peekCountLocked` — GET on first sight only. Called while `l.mu` is held.
- `windowcounter/limiter.go` `bufferedCountLocked` — GET on first sight of previous (or any unseen) key. Called while `l.mu` is held.
- `windowcounter/limiter.go` `getCount` — `redis.Get`; no lock of its own.
- `windowcounter/limiter.go` `flushPending` — `l.mu.Lock()` then `defer Unlock()` then `flushPendingLocked`.
- `windowcounter/limiter.go` `flushPendingLocked` — `redis.Eval` per window with `localDelta > 0` while `l.mu` is held; on success `redisKnown = n` and `localDelta = 0`.
- `windowcounter/limiter.go` `bufferedOutageErrorLocked` — `flushPendingLocked` and `getCount` while `l.mu` is held (Take/Peek outage probe).
- `windowcounter/limiter.go` `takeExact` / `peekExact` — Redis INCR/GET/EXPIRE with no `l.mu`.
- `windowcounter/fake_redis_test.go` `serve` GET — replies immediately; no `blockGetPrefix` / `waitGetBlocked` / `unblockGet`.
- Dest `windowcounter/` has no `repro_lock_during_get_test.go`.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — unit fake must kill sockets; does not require that Take not wait on another key's GET.
- `knowledge/devdocs/std_go_windowcounter.md` — first-sight GET / buffered Peek; does not document mutex across Redis I/O.

## Desired
1. Tests first. Create a failing repro on dest (port `blockGetPrefix` / `waitGetBlocked` / `unblockGet` from the parent fake; example path `windowcounter/repro_lock_during_get_test.go`). Confirm FAIL (~400ms wait on fast Take while GET on unrelated `slow` is held). Then unlock-around-I/O. Then that test PASSES and `go test -short -count=1 -timeout 60s ./windowcounter` passes.
2. Do not call Redis while holding `l.mu`.
3. `windowLocked` / first-sight Peek GET: snapshot under lock, unlock, `getCount`, re-lock, apply `redisKnown` (merge if another Take already inserted the key).
4. `flushPending`: copy `(key, delta, expireAt)` under lock, unlock, Eval, re-lock: `redisKnown = n`, `localDelta -= flushedDelta`. Do not set `localDelta = 0` blindly.
5. Exact mode unchanged.
6. Fast Take must finish well under the GET delay.

## Affected
- `windowcounter/limiter.go` (`takeBuffered`, `peekBuffered`, `windowLocked`, `peekCountLocked`, `flushPending` / `flushPendingLocked`; first-sight GET paths that today run under `l.mu`)
- `windowcounter/fake_redis_test.go` (GET hold helpers)
- New dest test proving HOLB then proving unlock-around-I/O
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` if propose adds a mutex-across-I/O invariant
- `knowledge/devdocs/std_go_windowcounter.md` if usage still implies GET under the Take lock

## Out of scope
- Bug 2: GET-refresh of an already-mapped previous window (`bufferedCountLocked` skip when the key is already in `l.windows`). Not this PR unless required to compile.
- Other parent `windowcounter/BUGS.md` items (buffered outage hide, Sleep/Wake hang, exact EXPIRE retry, fractional window, Peek vs next Take at limit).
- Changing exact-mode INCR/EXPIRE/GET.
- Rewriting `flushScript`.

## Unknowns
- Whether first-sight previous GET in `bufferedCountLocked` uses the same snapshot/unlock/merge as `windowLocked` (ticket names that helper only for current / Peek first-sight; the general rule is still no Redis under `l.mu`).
- Whether `bufferedOutageErrorLocked`'s probe GET / nested flush also drops `l.mu` before Redis (implied by the general rule; ticket merge recipe names `flushPending` only).
- Test file name on dest (caller example is `repro_lock_during_get_test.go`).

## Tensions
- Ticket: no Redis under `l.mu`. Dest `windowLocked` also GETs when `localDelta == 0` on an already-mapped current key. That is this HOLB, not bug 2. Bug 2 is previous already-in-map skip GET; leave that skip.
- Ticket: after Eval, `localDelta -= flushedDelta`, not `localDelta = 0`. Dest today zeros the delta. Ticket wins; a concurrent Take may have incremented during Eval.
- Spec does not mention mutex-across-I/O. Ticket wins; spec/usage catch up in propose if they still describe GET as part of the locked Take.
- Parent repro and fake GET-hold helpers exist only on the caller checkout, not dest. Port; do not assume dest already fails that test.
