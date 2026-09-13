# Requirement
IssueKey: 2026-09-13-simpleredis-idle-mutex-defer

## Problem
`release` locks `idleConnsMu` and unlocks by hand on two exit paths. A panic inside that section leaves the mutex locked forever. Every later `borrow` blocks in `takeIdleConn`, every `release` blocks, and `Close` blocks, while Traefik keeps the process up. That is worse than the in-use-turn leak PR #71 already defers around. Dest already defers `release` in `runOnConn` and `freeInUseTurn` in `borrow`; the idle mutex in `release` is the remaining hand-unwound resource.

## Current (code)
- `simpleredis/pool.go` `release` — `reusable == false` closes then `freeInUseTurn`. Reusable path stamps `conn.lastUsed`, locks `idleConnsMu`, computes `idleConnsFull` / `inUse` / `live` (comment: inUse still includes this socket until `freeInUseTurn` runs), unlocks then `conn.close()` + `freeInUseTurn` when closed or (`idleConnsFull && live >= liveCap()`), else appends, unlocks, `freeInUseTurn`. Two manual unlocks. No `parkIdleConn`.
- `simpleredis/pool.go` `borrow` / `dial` — dest already has `handedOff` plus deferred `freeInUseTurn` when not handed off (#71), and both return `(*pooledConn, error, bool)` with `handshakeFailed` (handshake-sentinel-match on dest). Do not change those signatures here.
- `simpleredis/pool.go` `takeIdleConn` — `Lock` + `defer Unlock`; returns `stale` for the caller to close after the lock.
- `simpleredis/pool.go` `freeInUseTurn` — extra send on a full `inUseTurns` is dropped and counted on `OverFrees`.
- `simpleredis/commands_exec.go` `runOnConn` — `reusable` starts false; `defer sr.release(conn, reusable)`. No `pull/29` comment.
- `simpleredis/resp.go` `do` — comment: `runOnConn` defers `release`. No "release never runs".
- `simpleredis/commands_msetex.go` `cachedGroupWrite` / `storeGroupWrite` — `groupWriteMu.Lock()`, one field read or write, manual `Unlock`.
- `simpleredis/simpleredis.go` `Close` — swaps `idleConns` under the lock, closes sockets after unlock.
- `simpleredis/yaegi_defer_test.go` `TestYaegi_DeferRunsOnPanic` — dest proof that interpreted defer runs for explicit panic, interpreter `errors.As`, and nil-map write.
- `simpleredis/BUGS.md` section 2 — still titled "One lost in-use turn permanently bricks the client", owned by `2026-09-13-simpleredis-lost-turn-recovery`, and says "`exec` releases without `defer`". The ticket quote "Do not simply switch to `defer sr.release(...)`; PR 29 rejected that…" is not found on dest.
- `knowledge/devdocs/std_go_simpleredis.md` — already has the Yaegi-defer-runs gotcha pointing at `simpleredis/yaegi_defer_test.go`. No stale "defer does not run" claim.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — already requires interpreted proof that a deferred restore runs under Yaegi v0.16.1 for those three panic classes. No stale "do not defer" claim.
- `simpleredis/resp.go` — `errors.Is` not a `net.Error` assert; `context.AfterFunc` not `go` + `select` on `ctx.Done()`.
- Retry jitter — `simpleredis/commands_exec.go` still `math/rand` (comment names Yaegi).

## Desired
- Extract the locked keep-or-close decision in `release` into a small method (`parkIdleConn`: sibling of dest `takeIdleConn`). That method locks, `defer`s unlock, decides, appends on success, returns bool. `release` acts after it returns: close when `!reusable || !parkIdleConn(conn)`, then `freeInUseTurn()` last on every path. Two `conn.close()` sites become one; three `freeInUseTurn()` sites become one.
- Do not `defer sr.idleConnsMu.Unlock()` at the top of `release`. Unlock must not span `conn.close()` or the `freeInUseTurn` channel send.
- Close decision unchanged: close when the client is closed, or when `idleConns` is already at `maxIdleConns` and `live >= liveCap()`. Otherwise park.
- `conn.lastUsed` still stamped before the socket is parked (dest stamps it on the reusable path before the lock, including when the later decision is close).
- `freeInUseTurn()` last on every path. Do not free the turn before or during the locked section (live-count arithmetic still includes this socket's turn).
- `OverFrees` unchanged: exactly one free per `release`, no double frees.
- No production hooks or seams only to unit-test a panic inside the critical section. Add a test only if it needs no production API change.
- `cachedGroupWrite` and `storeGroupWrite`: convert the two manual `groupWriteMu` unlocks to `defer`. Two-line consistency change.
- `simpleredis/BUGS.md` section 2: replace the stale "exec releases without defer" / lost-turn-recovery ownership (and the ticket's disproven "do not switch to defer" guidance if it reappears) with the measured finding: interpreted `defer` does run on panic under Yaegi v0.16.1 for an explicit interpreted panic, the interpreter `errors.As` panic, and a nil-map write. Point at `simpleredis/yaegi_defer_test.go`. Do not restate the probe.
- Grep `knowledge/devdocs/std_go_simpleredis.md` and openspec specs for the same stale claim; dest already has the finding — leave them unless a stale sentence remains.
- Verify dest already dropped the stale `resp.go` / `commands_exec.go` comments; fix only if #71 missed one.
- Later phases: full local suite including Yaegi tests, `go vet`, measured CI green, pool and release tests `-count=5`.
- Stacked PR: GitHub base `2026-09-13-simpleredis-panic-safe-release`; body states stacked on #71 and must merge after it.

## Affected
- `simpleredis/pool.go` — `release` + new `parkIdleConn`
- `simpleredis/commands_msetex.go` — `cachedGroupWrite`, `storeGroupWrite`
- `simpleredis/BUGS.md` — section 2
- Tests only if possible without production API change (`simpleredis/pool_test.go` or similar)

## Out of scope
- `Close` in `simpleredis/simpleredis.go`
- `takeIdleConn` returning stale sockets for close after the lock
- `resp.go` `errors.Is` vs `net.Error`; `context.AfterFunc` vs `go` + `select`
- `math/rand` retry jitter
- Changing `OverFrees` itself
- Production seams to panic inside the locked section
- Duplicating #71's `runOnConn` / `borrow` / `yaegi_defer_test.go` / `resp.go` comment work
- Merging or rebasing onto master while #71 is open
- Implementing product code in prepare

## Unknowns
- Whether a panic-inside-`parkIdleConn` test can be written with no production API change. Explore; default is no test rather than a seam.
- Merge order vs in-flight branches that also touch `pool.go` `release` (see Tensions). This ticket does not wait on them.

## Tensions
- Dest is unmerged PR #71. This branch and GitHub PR base must stay `2026-09-13-simpleredis-panic-safe-release`, not master. Merge after #71.
- A naive `defer Unlock()` at the top of `release` is a regression (mutex held across close / channel send). Extract `parkIdleConn`; do not flatten unlock onto `release`.
- Ticket quotes a BUGS.md sentence that dest does not have. Section 2 on dest is still stale in a different way (`exec` without defer; owner `lost-turn-recovery`). Replace that stale record with the Yaegi-defer finding and the proof test path.
- In-flight, same files, do not narrow this ticket: `2026-09-13-simpleredis-close-abandoned-socket` (likely `release`); `2026-09-13-simpleredis-desync-boundary-check` (`resp.go`); `2026-09-13-simpleredis-resilience-test-coverage`; `2026-09-13-golangci-lint-harden` (named results on `dial` / `borrow` in `pool.go`).
