# Requirement
IssueKey: 2026-09-12-simpleredis-risk-05-freeinuseturn-blocks-when-full

## Problem
`freeInUseTurn` sends on `inUseTurns` with no `default`. That channel is buffered at `PoolSize` and pre-filled to full at `New`. A second send with no matching receive parks the goroutine forever: no error, no log, no panic. Dest has no current double-free on the borrow/release paths; this is the failure mode of a future accounting bug (a cancelled command that returns early and is later released is the likely trigger).

## Current (code)
- `simpleredis/pool.go:54-59` — `freeInUseTurn` no-ops when `inUseTurns` is nil, else blocking `sr.inUseTurns <- struct{}{}`.
- `simpleredis/pool.go:41-51` — `ensureInUseTurns` makes a `liveCap`-buffered channel and pre-fills every slot.
- `simpleredis/simpleredis.go:51-53` — `closed atomic.Bool` and `inUseTurns chan struct{}`; no `overFrees` field.
- `simpleredis/simpleredis.go:101-108` — `PoolSize()` / `MaxIdleConns()` accessors; `OverFrees()` `not found`.
- `simpleredis/pool.go:61-106` — `borrow` receives one turn, then `freeInUseTurn` on closed-client or dial failure before returning error.
- `simpleredis/pool.go:127-154` — `release` always `freeInUseTurn` (dirty, closed/full idle, or idle-append).
- `simpleredis/commands_exec.go:18-27` — `exec` `borrow`s then `release`s; borrow error does not release.
- `simpleredis/pool_test.go` — reuse, concurrent cap, auth, idle timeout; no over-free or `len(inUseTurns)==cap` + counter invariant.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — live cap / wait / timeout; no over-free or non-blocking return of a turn.
- `knowledge/devdocs/std_go_simpleredis.md:68` — live cap and pool wait; no over-free hang.

## Desired
- `freeInUseTurn` uses `select` with `default`: a send that would block is dropped and `overFrees` increments (`atomic.Int64`, same Yaegi-safe pattern as `closed atomic.Bool`).
- Export `OverFrees() int64` next to `PoolSize()` / `MaxIdleConns()`.
- Guard test: `freeInUseTurn` on a fresh client returns promptly and `OverFrees()` is 1.
- Invariant test: hammer borrow/release exits (healthy fake, dead address, AUTH reject, starved pool with a short `PoolTimeout`) from many goroutines; assert `len(inUseTurns) == cap(inUseTurns)` and `OverFrees() == 0`.
- Correct accounting stays silent (`OverFrees() == 0`); no behaviour change on that path.

## Affected
- `simpleredis/pool.go` (`freeInUseTurn`)
- `simpleredis/simpleredis.go` (`overFrees` field, `OverFrees()`)
- `simpleredis/pool_test.go` (or a new dest test file this change adds)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (over-free must not hang; counter)
- `knowledge/devdocs/std_go_simpleredis.md` (gotcha: over-free used to hang)

## Out of scope
- Other `simpleredisfixes2/` findings (bug-01 panic leak, bug-03 MaxIdleConns, risk-01 context cancel, ci-01 race detector in CI).
- Replacing the channel-as-semaphore with an explicit counter under `idleConnsMu`.
- Panic on over-free.
- Logs or Traefik metrics besides `OverFrees()`.
- Pipelining, idle reaper, or other pool refactors.

## Unknowns
- Whether the tcp-session spec names the `select`/`default` path or only the tests prove it.
- Whether `OverFrees()` is compiled-test only or also observed from `e2e/simpleredisprobe` (ticket says a test or an operator can read the method).
- `-race` in CI is a sibling finding; this ticket can still run the invariant under `-race` locally.

## Tensions
- Ticket rates this judgement because dest accounting is balanced today; the ask is still to change how a future double-free manifests.
- Ticket lists panic as louder; the how-to-fix is silent drop plus counter. Follow the how-to-fix, not panic.
- Belt-and-braces mutex counter would also touch bug-03's `len(inUseTurns)` read; this run stays on the two-line `select`/`default`.
