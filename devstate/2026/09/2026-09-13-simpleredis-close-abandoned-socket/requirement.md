# Requirement
IssueKey: 2026-09-13-simpleredis-close-abandoned-socket

## Problem
Lost-turn recovery after a panic between `borrow` and `release` restores concurrency and dials a new socket, but never closes the abandoned fd. The client stays usable and accumulates one dead fd per panic, unbounded. A plugin that panics once an hour leaks about 24 fds a day; fd exhaustion is the same outage class as the turn brick, slower.

## Current (code)
- `simpleredis/commands_exec.go` `exec` — `borrow`, then `doWithHeldSocket`, then `release`. No `defer release`. Comment cites PR #29.
- `simpleredis/commands_exec.go` `doWithHeldSocket` — `heldSockets.Add(1)` then `defer Add(-1)` around `do`. Covers the socket only while `do` runs. Panic inside `do` restores the count; panic between `borrow` and `doWithHeldSocket` leaves it at 0.
- `simpleredis/pool.go` `takeIdleOrDial` — idle miss dials while `heldSockets` is incremented around `dial` (AUTH/SELECT panic restores the count). `borrow` returning a reused idle conn does not increment `heldSockets`.
- `simpleredis/pool.go` `borrowAfterPoolWait` / `recoverLostTurnsLocked` — on pool-wait timeout, if `heldSockets == 0` and `len(idleConns) == 0`, refill `inUseTurns` and increment `lostTurns`. Does not hold or close any `*pooledConn`. Next command dials a new socket.
- `simpleredis/simpleredis.go` — `heldSockets` is `atomic.Int64` (a count). `LostTurns()` is the restored-token counter. No checkout registry, no per-conn timestamp, no lease table.
- `simpleredis/pool.go` `pooledConn` — `netConn`, reader, writer, `lastUsed`. No checkout time. After a panic, the pointer is unreachable from the client.
- `simpleredis/pool.go` `release` — reuse path publishes to `idleConns` then `freeInUseTurn`; destroy path `close()` then `freeInUseTurn`. Neither path is reached when `exec` panics after `borrow`.
- `simpleredis/commands_exec.go` `bindCommandDeadline` — library budget is `(maxRetries+1)*(DialTimeout()+IOTimeout())`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` “Lost in-use turns MUST NOT brick the client” — restore turns when owned sockets are zero; `exec` MUST NOT defer `release`; `New` MUST NOT start a recovery goroutine. Scenario “Recovered panics after borrow do not brick the client” asserts Get succeeds and `LostTurns() >= PoolSize`, not that abandoned fds close.
- Same spec “Idle connections are pooled” — a socket between borrow and do, or between do and release, is not counted as owned; refill MAY briefly overshoot `PoolSize`.
- `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md` — this leak; recovery dials instead of reclaiming the fd.
- `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md` — the two gaps where idle is empty and `heldSockets` is 0 while a socket is still live.
- `devstate/2026/09/2026-09-13-simpleredis-lost-turn-recovery/issues.md` — both debt rows `[ ]`.
- `simpleredis/pool_test.go` `TestBugLostInUseTurnBricksPoolPermanently` — `bugPanicAfterBorrow` then Get succeeds; `LostTurns() >= PoolSize`. Uses `fake.connections()` (accept count) in the failure message; does not assert `openSockets()` settles.
- `simpleredis/pool_test.go` `bugPanicAfterBorrow` — `borrow` then panic; caller `recover()`. Ported helper.
- `simpleredis/pool_test.go` `TestRecoveryDoesNotFireWhileSocketsBusy`, `TestLostTurnsReportsLeak`, `TestDoWithHeldSocketPanicRestoresHeldCount`, `TestRecoveryDoesNotFireDuringHungDial` — green on DestBranch.
- `simpleredis/pool_test.go` `TestPoolWaitTimesOutWithoutExtraDial`, `TestOverFreeAccountingStaysBalanced`, `TestConnectionIsReused` — green; last is 25 sequential Gets on exactly 1 TCP accept.
- `simpleredis/fake_redis_test.go` `connections()`, `openSockets()`, `waitOpenSocketsEqual` — accept count vs still-open sockets.
- `simpleredis/` source — no checkout `go` goroutine in non-test files. `resp.go` comments Yaegi `interp._select` on a context channel.
- Checkout registry / set / lease table of `*pooledConn` — not found.

## Desired
- Close the abandoned socket after a panic between `borrow` and `release`. Recovery must not leave that fd open for the process lifetime. Server-side open sockets settle at a bound, not only turns recovering.
- Keep a checked-out `*pooledConn` reachable from the client (registry / set / lease table), not only `heldSockets`. Register on `borrow`; deregister in `release` (reuse and destroy).
- Reclaim only from the existing pool-wait recovery path. Do not add a background sweeper goroutine (Yaegi; `resp.go` already avoids selecting on a context channel from a goroutine).
- Reclaim only conns whose checkout is older than a generous bound derived from the command budget already computed: `(MaxRetries+1)*(DialTimeout+IOTimeout)` (`bindCommandDeadline`). Prefer late reclaim over early. A live in-flight command must complete correctly with its socket untouched.
- A conn in the borrow-to-do or do-to-release gap must not be reclaimed.
- `LostTurns()` (and any new counter) still reports honestly.
- If the same registry also closes the held-socket-lease windows so `recoverLostTurnsLocked` no longer needs the bare `heldSockets` count, take that and close that dest debt row (`[x]`, `Taken:`, delete `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`). If it does not fall out, leave it.
- Preserve: no deferred `release` in `exec` (PR #29); `PoolSize` remains the live-socket cap with only #66's documented bounded transient overshoot; `TestOverFreeAccountingStaysBalanced` ends `len(inUseTurns)==cap` and `OverFrees()==0`; `TestPoolWaitTimesOutWithoutExtraDial` still `redis:unreachable` within one PoolTimeout and no extra dial; #66 tests stay green; `TestConnectionIsReused` stays 25 Gets / 1 TCP.
- Permanent untagged default-suite tests: (1) after `PoolSize` recovered panics, abandoned sockets eventually close (`openSockets` / `waitOpenSocketsEqual`, not just turns); (2) a live in-flight command past a pool-wait recovery on another goroutine is never reclaimed (run at `-count=5`); (3) a conn in the borrow-to-do or do-to-release gap is not reclaimed; (4) `LostTurns()` (and any new counter) still reports honestly.
- Package constraints from `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: stdlib-only source; no `unsafe`, cgo, generics; jitter `math/rand` `Int63n`; Yaegi-safe (no `errors.As` on package-local struct, no `net.Error` asserts, no extra goroutines); pool/timeout/retry knobs frozen at New; new read-only accessors are fine.
- This PR stacks on #66. #66 must merge first. Dest is `2026-09-13-simpleredis-lost-turn-recovery`, not `master`.

## Affected
- `simpleredis/pool.go` — checkout registry, register/deregister, reclaim on pool-wait recovery (not handshake / dial classification)
- `simpleredis/commands_exec.go` — only if reclaim age uses `bindCommandDeadline` budget, or if lease windows close
- `simpleredis/simpleredis.go` — registry field; `LostTurns()` stays; optional new read-only counter
- `simpleredis/pool_test.go` (and/or a new `_test.go`) — close-abandoned, live-command, gap, honesty tests
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — recovery currently does not require closing the abandoned fd
- `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md` — this ticket takes that note
- `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md` — only if the registry closes those windows

## Out of scope
- `simpleredis/BUGS.md` (PR #68 owns it)
- Merging or cherry-picking PRs #67, #68, #69
- `defer sr.release` in `exec` (PR #29 stands)
- A background sweeper goroutine
- Closing a socket on the naive predicate “checked out, not in idle, no active command” without an age bound
- Chasing the held-socket-lease debt if it does not fall out of the fd close
- Writable pool/timeout/retry knobs; third-party / vendored imports; `unsafe` / cgo / generics; `math/rand/v2`

## Unknowns
- Whether a timestamped checkout registry also closes the two `heldSockets` gaps so the bare count can go. Ticket: take only if it falls out; otherwise leave the debt file.
- Name and placement of any new read-only counter besides `LostTurns()`.

## Tensions
- False-positive close of a live command is a production incident (fd reuse / read on closed socket). The leak is slow degradation. Ticket: be conservative; late reclaim is harmless.
- The naive “not in idle and no active command” predicate is also true in the two #66 gaps (`commands_exec.go` comments; `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`). Closing on that predicate alone will close live sockets.
- Dest recovery is allowed to overshoot `PoolSize` in those gaps (`openspec/specs/std_go_simpleredis_tcp-session/spec.md`). Closing fds must not turn that window into a live-command kill.
- This work stacks on unmerged PR #66 (`2026-09-13-simpleredis-lost-turn-recovery`). `master` is not dest. `origin/HEAD` points at stale `origin/initial`.
