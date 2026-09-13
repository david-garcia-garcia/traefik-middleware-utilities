# Requirement
IssueKey: 2026-09-13-simpleredis-lost-turn-recovery

## Problem
A panic between `borrow` and `release` drops one in-use turn forever. After `PoolSize` recovered panics the client returns `redis:unreachable` on every command with a healthy Redis and zero live sockets. Traefik recovers a panicking middleware per request, so the process stays up and the client dies quietly.

## Current (code)
- `simpleredis/commands_exec.go` `exec` — `borrow`, then `do`, then `release` (and on `contextStop`, `release` with `reusable=false`). No `defer`. Comment: Yaegi panic loses the turn; not deferred-release; cites https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/29.
- `simpleredis/pool.go` `ensureInUseTurns` — `New` builds `inUseTurns` once, buffered to `liveCap()`, filled once. Nothing refills missing tokens. No reaper, no generation, no idle-sweep recovery.
- `simpleredis/pool.go` `borrow` — takes from `inUseTurns`; waiter past cap uses `PoolTimeout` then `errPoolWait`. No check of idle sockets or of sockets actually owned. `liveCap()` is `cap(inUseTurns)`.
- `simpleredis/pool.go` `release` / `freeInUseTurn` — extra send when the channel is already full is dropped and counted on `overFrees`. Missing returns are not restored.
- `simpleredis/simpleredis.go` `OverFrees()` — read-only extra-return counter. `LostTurns()` not found.
- `simpleredis/pool.go` `release` — `inUse` is `liveCap() - len(inUseTurns)`. After a leak that figure is not “sockets the client owns”.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` “Idle connections are pooled” — live sockets (idle plus checked out) SHALL not exceed `PoolSize`; when idle is empty and live sockets are at `PoolSize`, wait instead of dialing. “Full pool wait returns redis:unreachable” — busy at cap, one `PoolTimeout`, no extra TCP.
- Same spec “Extra in-use turn return MUST NOT hang” — `OverFrees()` next to `PoolSize()` / `MaxIdleConns()`; balanced borrow/release leaves `len(inUseTurns)==cap` and `OverFrees()==0`.
- `simpleredis/pool_test.go` — `TestOverFreeAccountingStaysBalanced`, `TestPoolWaitTimesOutWithoutExtraDial`, `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestOverFreeOnFullSemaphoreReturns`.
- `simpleredis/commands_deadline_test.go` `TestGetCancelWhileWaitingForTurn` — cancelled waiter does not take a turn; holder keeps the only token.
- `simpleredis/fake_redis_test.go` `pooledIdle` — idle-list length under `idleConnsMu`.
- `simpleredis/commands_exec.go` `isHandshakeFailure` — type assert, not `errors.As` (Yaegi panic on `As` for this struct).
- `simpleredis/resp.go` `ioError` — `errors.Is`, not `net.Error` (Yaegi panic). `context.AfterFunc` comment: Yaegi `interp._select` on a context channel.
- `simpleredis/resp.go` `readBulk` / `parseLen` — `length > maxBulkLength` (64<<20) returns `errIssue` before `make([]byte, length+2)`.
- `simpleredis/BUGS.md` — not found on `origin/master`. `origin/bugfixes20260913:simpleredis/BUGS.md` section 2 is this bug (latent; `readReply` fuzz 7.04M execs / 91s, no panic).
- `simpleredis/bugs_repro_test.go` — not found on `origin/master`. `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` (`//go:build bugrepro`) has `TestBugLostInUseTurnBricksPoolPermanently` and `bugPanicAfterBorrow` (`borrow` then `panic`, caller `recover()`).
- PR 29 — closed, not merged. Owner comment: discarded defer-release as too much complexity for Yaegi recovering the request while the turn stays lost. Pointer comment next to `sr.do` is the rejected approach.

## Desired
- Do not switch `exec` to `defer sr.release(...)`. PR 29 rejected that. Argue on the delivery card only if later work still believes defer is correct, against that discard comment.
- Fix permanence. Cheapest sufficient: when `borrow` is about to return `errPoolWait`, if the client owns no socket (`len(idleConns)` plus a count of dialed-but-not-yet-released sockets) and free turns are zero, refill to the frozen cap and count on a new `LostTurns()` metric. Or make the turn a leased resource so a lost lease is recoverable.
- Recovery MUST NOT fire while sockets are genuinely live and busy. Distinguish “no live sockets” from “all live sockets busy”. `PoolSize` stays the frozen live-socket cap.
- Expose `LostTurns()` next to `OverFrees()` in `simpleredis/simpleredis.go`. Read-only. No new writable Config/client field.
- Preserve: `TestOverFreeAccountingStaysBalanced` ends `len(inUseTurns)==cap` and `OverFrees()==0`; `TestPoolWaitTimesOutWithoutExtraDial` still `redis:unreachable` within one `PoolTimeout` and no extra dial; keep green `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestGetCancelWhileWaitingForTurn`, `TestOverFreeOnFullSemaphoreReturns`.
- Permanent, untagged tests in the default suite: (1) port `TestBugLostInUseTurnBricksPoolPermanently` so it PASSES after the fix (`borrow` then panic, `recover()` in the caller); (2) saturated pool with live in-flight commands still caps at `PoolSize`, still pool-wait `redis:unreachable`, never dials past the cap; (3) `LostTurns()` (or the exposed leak count) reports the leak.
- Reuse the `bugrepro` helper as the starting point. Do not create or edit `simpleredis/BUGS.md`.
- Package constraints from `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: stdlib-only source; no `unsafe`, cgo, or generics; jitter `math/rand` `Int63n`; keep Yaegi workarounds; prefer fields/atomics/mutexes on the client.

## Affected
- `simpleredis/pool.go` — turn accounting in `borrow` / live-socket count (not `handshakeFailure`, not `dial` error classification)
- `simpleredis/simpleredis.go` — `LostTurns()` next to `OverFrees()`
- `simpleredis/` default-suite tests (port of `TestBugLostInUseTurnBricksPoolPermanently` plus busy-pool and `LostTurns()` cases)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — wait-not-dial is backpressure only when sockets are actually live at `PoolSize`

## Out of scope
- `simpleredis/BUGS.md` (another agent owns it)
- `simpleredis/resp.go`
- `handshakeFailure` type, `isHandshakeFailure`, and `shouldRetry` classification (concurrent rewrite of handshake bool out of `dial` through `borrow`)
- `defer sr.release` as the fix (PR 29 discarded)
- BUGS.md bugs 1 and 3 (handshake matcher under Yaegi; desynced socket)
- Writable pool/timeout/retry knobs; go-redis / miniredis / vendor imports; `unsafe` / cgo / generics; `math/rand/v2`

## Unknowns
- Refill-on-`errPoolWait` vs a leased turn: ticket prefers cheapest sufficient if it preserves the live-socket cap.
- Name and placement of the dialed-but-not-yet-released counter (ticket requires the distinction; dest has no such field).

## Tensions
- PR 29 discarded defer-release (“too much complexity for Yaegi recovering the request while the in-use-turn stays lost”). Ticket agrees: fix permanence, not defer. Later work that still wants defer must argue against that comment, not ignore it.
- Spec wait-not-dial assumes live sockets are at `PoolSize`. Dest `borrow` treats an empty `inUseTurns` as that cap even when idle is 0 and no socket is owned — that is a dead client, not backpressure.
- Severity is latent, not a live compiled crash: ticket + BUGS.md Rejected (`readReply` 7.04M execs / 91s, `parseLen` capped by `maxBulkLength`). Yaegi panics are already documented in this package, and Traefik per-request recover makes each one permanent.
