# Requirement
IssueKey: 2026-09-13-simpleredis-idle-socket-reaper

## Problem
`IdleTimeout` is documented as an idle reuse gate, but the sweep that closes aged sockets runs only inside `takeIdleConn`, which runs only when a command borrows. With traffic stopped, idle sockets and their fds stay open past `IdleTimeout`. Measured (untracked hunt): `IdleTimeout` 50 ms, 500 ms of silence, idle list 4 and server-side open sockets 4. Standalone this is fd and Redis `maxclients` pressure per plugin instance. It also leaves the sockets that a server-side Redis `timeout` (or restart) will drop, which a sibling defect (BUG-1, stale pooled socket retry) then feeds to the next request as guaranteed failures.

## Current (code)
- `simpleredis/pool.go` `takeIdleConn` — under `idleConnsMu`, filters `idleConns` by `now.Sub(conn.lastUsed) < sr.idleTimeout`, returns stale for close after unlock, then LIFO-pops one survivor. Called only from `borrow`. No other call site.
- `simpleredis/pool.go` `borrow` — takes an in-use turn, calls `takeIdleConn`, closes `stale` after unlock, hands off a reused socket or `dial`s. Does not validate a reused socket beyond the age sweep. BUG-1 sibling work also lives on this path; this ticket must not change reuse/validation/reporting of a borrowed socket.
- `simpleredis/pool.go` `release` / `parkIdleConn` — stamps `lastUsed = time.Now()` on the reusable path, then parks if open and `len(idleConns) < maxIdleConns`. Stamp is relative (`lastUsed`), not an absolute expiry.
- `simpleredis/simpleredis.go` `New` — copies `Config`, `ensureInUseTurns()`, returns. Starts no goroutine. `TestStaleIdleHeadIsClosedWhileTailStaysHot` in `simpleredis/pool_test.go` asserts `runtime.NumGoroutine()` is unchanged across `New`.
- `simpleredis/simpleredis.go` `Close` — CAS `closed`, drains `idleConns` under the mutex, closes those sockets. Idempotent. No reaper to stop. `simpleredis/not_from_new_test.go` calls `Close` twice.
- `simpleredis/simpleredis.go` `IdleTimeout()` / `simpleredis/config.go` — godoc and `Config.IdleTimeout` comment: how long an idle socket may sit before **borrow refuses to reuse it**. Default 30s.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` Requirement "Idle connections are pooled" — on the next borrow, every idle socket older than `IdleTimeout` SHALL be closed, including a stale head behind a younger tail. `New` MUST NOT start a goroutine to close idle sockets. A client that issues no later command MAY keep idle sockets past `IdleTimeout` until `Close`. Scenario "Stale idle head is closed while the tail stays hot" also asserts `New` did not start a goroutine. Landed by archived `openspec/changes/archive/2026-09-12-simpleredis-idle-borrow-sweep/` (PR #33).
- `knowledge/devdocs/std_go_simpleredis.md` gotcha — borrow sweeps aged idle sockets; `New` starts no reaper; a fully quiet client keeps those sockets until `Close`.
- `simpleredis/lifecycle_test.go` `TestLifecycleNewUseCloseDoesNotLeak` — 200 `New`/use/`Close` cycles; goroutine count must stay within +2; server-side open sockets 0.
- `simpleredis/resp.go` `watchConnClose` — Yaegi v0.16.1 `interp._select` races when interpreted code selects on a context channel from a goroutine; dest uses `context.AfterFunc` instead of `go` + `select` on `ctx.Done`.
- `e2e/simpleredisprobe/plugin.go` Traefik `New` — `simpleredis.New` only; no `client.Close`.
- `windowcounter/limiter.go` `Close` — "does not close the injected SimpleRedis."
- `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md` — previous ticket `2026-09-12-simpleredis-risk-02-no-idle-reaper` noted a background ticker is only safe if something stops it, and dest product `New` paths do not call `SimpleRedis.Close`.
- `knowledge/research/ext_go-redis_connection-pool/notes.md` — go-redis `ConnMaxIdleTime` (default 30 minutes) is checked lazily on the next `Get` (`isHealthyConn`), not by a background reaper.
- `knowledge/research/ext_redis_clients_idle-close/notes.md` — Redis `timeout` default 0 (never close idle clients). Dest compose/CI `redis:7-alpine` ships `timeout 0`. A positive `timeout` closes a normal idle client; the check is incremental.
- Untracked hunt (caller workspace, not dest): `simpleredis/PRODUCTION-BUGS.md` BUG-6; `simpleredis/bugs_production_test.go` `TestBugIdleSocketsAreNeverReapedWithoutTraffic` (`//go:build simpleredis_bugs`). Dump pointer: `ticket/bug-6-hunt.md`.
- Background idle reaper in dest `simpleredis` — not found.

## Desired
- Unattended run, but the simplicity gate wins: if the simplest correct fix is not small, coherent, and elegant, stop after propose with options and costs written; that stop is success. Pick at most one of the three directions.
- Direction 1: background reaper (ticker goroutine started in `New`, stopped in `Close`). Only this truly releases fds during silence. Cost: every client in a Traefik plugin process gains a goroutine; `New`'s contract becomes "starts a goroutine"; forgetting `Close` leaks. `TestLifecycleNewUseCloseDoesNotLeak` and `TestStaleIdleHeadIsClosedWhileTailStaysHot` must stay green (`New`/use/`Close` goroutine count flat). Yaegi: read `resp.go` `watchConnClose` before adding a goroutine or `select`.
- Direction 2: stamp an absolute expiry at park so the reuse gate and socket lifetime agree, with no background work. Does not release fds during silence; say so if chosen.
- Direction 3: fd pinning is acceptable; the real fix is the sibling BUG-1 (validate on borrow); document dest as specified today.
- If a product fix lands: only `simpleredis`. Go 1.21, stdlib only, Yaegi-interpretable. Comment style: constraint or reason, never restate the code. `Close` stays idempotent and must not race a reaper. In-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks. Diff surgical to reaping; do not change how `borrow` validates or reports a reused socket (BUG-1 sibling).
- If a product fix lands: a permanent **untagged** test that parks several sockets, stops traffic, and after a multiple of `IdleTimeout` asserts both the idle list and the server-side open socket count have dropped, plus goroutine count back to start after `Close`. Prefix new fakes/helpers with something bug-specific so they cannot collide with sibling branches.
- Verify (if implementing): `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`, Yaegi tests, and the tagged hunt reproduction.

## Affected
- `simpleredis/pool.go` (reap path only, if a code fix)
- `simpleredis/simpleredis.go` `New` / `Close` (only if direction 1)
- `simpleredis/*_test.go` new untagged test (if implementing)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (direction 1 would reverse "MUST NOT start a goroutine" / "MAY keep until Close"; direction 3 would leave it)
- `knowledge/devdocs/std_go_simpleredis.md` idle gotcha (if the contract changes or is restated)

## Out of scope
- Any package other than `simpleredis` (including wiring `reclaim.Hooks{Close}` or `windowcounter`/`tokenbucket` to call `SimpleRedis.Close`)
- BUG-1 (stale pooled socket retry) and other hunt bugs (BUG-2..5)
- Changing how `borrow` validates or reports a reused socket
- Committing untracked `simpleredis/PRODUCTION-BUGS.md` or `simpleredis/bugs_production_test.go`
- Editing `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md` unless this run takes that follow-up (it is a prior note, not this ticket's product tree)
- Writable pool/timeout knobs; third-party session imports; `unsafe` / cgo / generics
- Implementing product code in prepare

## Unknowns
- Production Redis `timeout` is not measured here. Dest compose is `timeout 0`. The amplifier claim needs a non-zero server timeout (or a restart/CLIENT KILL) to turn pinned sockets into corpses.
- Caller invoke used `-run 'TestBugIdleSocketsAreNeverReaped'`; the untracked file's function is `TestBugIdleSocketsAreNeverReapedWithoutTraffic`.
- Whether Traefik ever `Close`s a plugin-held SimpleRedis. Dest probe and windowcounter do not. Direction 1's leak-on-forgotten-Close is the production path in this tree.

## Tensions
- Dest spec and usage gotcha **encode** today's behavior as required (`New` MUST NOT start a reaper; quiet client MAY keep sockets until `Close`). Direction 1 reverses that. Direction 3 is dest as specified. Closed PR #33 / archived `simpleredis-idle-borrow-sweep` chose sweep-on-borrow and explicitly "Do not start a reaper goroutine."
- Prior ticket `2026-09-12-simpleredis-risk-02-no-idle-reaper` already declined a SimpleRedis ticker because dest callers never `Close`; it filed `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md`. This ticket forbids touching those callers.
- go-redis (the shape dest copies) also evicts idle on the next `Get`, not with a background reaper.
- Option 1 is the only direction that releases fds during silence and has the highest complexity cost (goroutine, `Close` race, Yaegi, spec reversal, forgotten-`Close` leak on dest callers). The simplicity gate says not to implement a fix that is not small/coherent/elegant.
- Simplicity gate overrides unattended "Done when": stopping after propose with a written recommendation is success.
- BUG-1 sibling branch `2026-09-13-simpleredis-stale-pooled-socket-retry` also edits `pool.go` borrow paths; keep this diff off that validation.
- Caller snippet of `takeIdleConn` has a typo (`append(survivors, append(survivors, conn)`). Dest is `survivors = append(survivors, conn)`.
- `origin/HEAD` is stale `origin/initial`. Dest is `master`.
