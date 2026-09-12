# Requirement
IssueKey: 2026-09-12-simpleredis-bug-01-panic-leaks-pool-token

## Problem
A panic between `borrow` and `release` permanently drops one in-use-turn token. After `PoolSize` such panics the semaphore is empty, later commands wait the full `PoolTimeout` and then return `redis:unreachable`, and the client never refills. Restarting Redis does not recover it.

## Current (code)
- `simpleredis/commands_exec.go` `exec`: after a successful `borrow`, the only return of the token is `sr.release(conn, reusable)` on the line after `sr.do`. There is no `defer`.
- `simpleredis/pool.go` `release`: sole caller of `freeInUseTurn` after a command holds a socket. `freeInUseTurn` is a blocking send onto `inUseTurns`.
- `simpleredis/pool.go` `borrow`: takes one token from `inUseTurns` before idle reuse or `dial`. On dial error it calls `freeInUseTurn`; on success it returns the socket still holding the token.
- `simpleredis/resp.go` `do` / `readReply`: `make([][]byte, count)` after `parseLen` accepts a non-negative length. `parseLen` rejects overflow past `maxParseLen`; a length that fits in `int` but exceeds `makeslice` still panics (ticket example `*1000000000000000000\r\n` on 64-bit).
- `simpleredis/config.go`: `defaultPoolSize` 8, `defaultPoolTimeout` 200ms.
- `simpleredis/pool_test.go` and `simpleredis/fake_redis_test.go` `startStaticRedis`: pool reuse and canned-reply tests. No test that recovers a panic in `do` and asserts `len(inUseTurns) == cap(inUseTurns)`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: live-cap and release rules; no panic-unwind scenario.
- Ticket-cited `simpleredisfixes/test-02-idle-cap-and-release-after-close.md` — not found on dest.

## Desired
1. After a successful `borrow` in `exec`, return the token even when `do` panics. On that panic path, treat the socket as not reusable (`reusable` left false) so it is closed, not pooled.
2. Do not recover the panic in `exec`. Let it propagate.
3. Prove the invariant: after `PoolSize` recovered panics from `do`, `len(inUseTurns) == cap(inUseTurns)` and a later `borrow` still succeeds.

## Affected
- `simpleredis/commands_exec.go` (`exec`)
- `simpleredis/pool.go` (`release` / `freeInUseTurn` as the token return path already in use)
- Pool tests next to `simpleredis/pool_test.go` (or `commands_exec_test.go`); canned panic reply via `startStaticRedis`
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` if propose adds the unwind scenario

## Out of scope
- Capping parser allocations so the panic cannot happen (`bug-02-unbounded-reply-allocation.md`).
- Making `freeInUseTurn` non-blocking (`risk-05-freeinuseturn-blocks-when-full.md`).
- Window-counter fail-open/fail-closed when `Take` errors (`bug-05-windowcounter-hides-outage.md`).
- Pipelining (`simpleredisfixes/perf-04-pipelining.md`).
- Recovering the panic in `exec` and mapping it to `redis:issue?`.
- Wrapping `do` inside `dial` AUTH/SELECT (token already taken; ticket’s defer is on `exec` after `borrow` returns).

## Unknowns
- Whether Traefik recovers a plugin panic per request so the process keeps serving (ticket claim; not measured in this tree; `knowledge/research/index_ext_traefik.md` has no panic-recovery finding).
- Whether Yaegi interpretation of a deferred `release` matches stdlib `defer` (ticket claims one deferred call per command).
- Exact test file once propose lands: dest has `pool_test.go`, not the ticket’s `test-02` markdown.

## Tensions
- Ticket ranks this with bug-02 as the live panic source; this run is only the unwind leak. Parser still panics on a huge-but-legal array length (`simpleredis/resp.go`).
- Ticket “consider” non-blocking `freeInUseTurn` while here; that is risk-05, not this ask.
- Ticket’s defer wraps `do` after `borrow` returns; `dial` still calls `do` for AUTH/SELECT while holding the token (`simpleredis/pool.go`). Same leak if those panic. Ticket did not ask to wrap `borrow`.
