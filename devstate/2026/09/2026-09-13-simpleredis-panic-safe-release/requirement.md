# Requirement
IssueKey: 2026-09-13-simpleredis-panic-safe-release

## Problem
`exec` releases a borrowed socket without `defer`. A panic between `borrow` and `release` loses that in-use turn forever and leaks the socket fd. `inUseTurns` is filled once in `New` and is never refilled, so after `PoolSize` recovered panics every command returns `redis:unreachable` against a healthy Redis with zero open sockets. Traefik recovers a panicking middleware per request, so the process survives and each panic silently spends one turn. PR #29 closed on the belief that a deferred `release` would not run under Yaegi; that belief is false on Yaegi v0.16.1.

## Current (code)
- `simpleredis/commands_exec.go` `exec` — after `borrow`, calls `do` then `release` with no `defer`. Comment cites PR #29: "If do panics under Yaegi, the process does not crash and this in-use-turn is lost."
- `simpleredis/pool.go` `borrow` — takes from `inUseTurns`, then `takeIdleConn` / `dial`. Explicit `freeInUseTurn()` only on the closed-idle and dial-error returns. A panic in `takeIdleConn` or `dial` after the take is not covered. No `handedOff` flag.
- `simpleredis/pool.go` `freeInUseTurn` / `ensureInUseTurns` — buffered channel sized to `PoolSize`; extra send is dropped and counted on `OverFrees`. Nothing refills a lost turn.
- `simpleredis/simpleredis.go` `New` / `SimpleRedis.inUseTurns` — channel created once; no `heldSockets`, `lostTurns`, `turnRecoverMu`, `LostTurns()`, `recoverLostTurnsLocked`, or `borrowAfterPoolWait`.
- `simpleredis/pool.go` `release` — destroy path `close()` then `freeInUseTurn`; reuse path publishes to idle then `freeInUseTurn`. Unreachable if `exec` panics after `borrow`.
- `simpleredis/resp.go` `do` — comment: "Any panic here loses the in-use-turn when this client runs in a Traefik middleware: Traefik recovers the request and release never runs."
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — idle pool, live cap, extra-turn `OverFrees`, close-on-release. No requirement that a panic returns the turn and closes the socket.
- `knowledge/devdocs/std_go_simpleredis.md` — Yaegi gotchas (`unsafe`, AfterFunc, conversion matrix). No gotcha that interpreted `defer` runs on panic.
- `simpleredis/BUGS.md` section 2 — records this brick; names closed branch `2026-09-13-simpleredis-lost-turn-recovery`. This ticket must not edit that file (#68 owns it).
- `simpleredis/pool_test.go` — reuse, live-cap, pool-wait, `OverFrees` balance. No panic-in-`do` turn-return test.
- `simpleredis/yaegi_test.go`, `simpleredis/yaegi_errorpath_test.go` — interpreted New/verbs and error paths. No defer-on-panic probe.
- `runOnConn` / scratch `zz_` tests on dest — not found.
- `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md`, `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md` — not found on dest (obsolete on closed #66).
- Prototype on `origin/proto-defer-net` (based on master): `simpleredis/commands_exec.go` `runOnConn` with deferred `release` and `reusable` starting false; `simpleredis/pool.go` `borrow` `handedOff` + deferred `freeInUseTurn` (explicit error-path frees removed); `simpleredis/zz_proto_defernet_test.go`; `simpleredis/zz_scratch_deferpanic_test.go`. Requester measured `turns=2/2 idle=0 accepts=3 OverFrees=0` after two recovered panics; `go test -count=1 -timeout 300s ./simpleredis/` green in 11.89s.

## Desired
- Keep the prototype design. Do not redesign. Argue a change on the delivery card with a measurement.
- `commands_exec.go`: move the loop's `do` + `release` into `runOnConn` with deferred `release`. `reusable` declared before the `defer`, initialised false (covers "do said not reusable" and "do never returned" — both destroy the socket). Assignment `values, reusable, err = sr.do(...)` (plain `=`; `values` and `err` named returns) so the closure sees the update.
- `pool.go` `borrow`: `handedOff` plus deferred `freeInUseTurn()` on every path that does not hand a socket to the caller, including a panic in `takeIdleConn` or `dial`. Remove the two explicit `freeInUseTurn()` calls on those error paths; keeping both double-frees and inflates `OverFrees`.
- Permanent tests, package naming (no `zz_`, `proto`, `scratch` in files or test names): panic-in-`do` via `bufio.Writer` over a panicking `io.Writer` so `conn.netConn` stays a healthy socket `release` can close (`simpleredis/pool_test.go` or new `simpleredis/panic_safety_test.go`). Yaegi defer-on-panic probe as a permanent test (`simpleredis/yaegi_defer_test.go`) — evidence that overturns PR #29.
- Update the stale `do` comment in `resp.go`.
- Update `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: in-use turn returned and socket closed even when the command panics.
- Gotcha on `knowledge/devdocs/std_go_simpleredis.md`: interpreted `defer` runs under Yaegi for explicit panic, interpreter `errors.As` panic, and nil-map write.
- Package constraints unchanged: stdlib-only session source; no `unsafe`, cgo, generics; jitter `math/rand` `Int63n`; Yaegi workarounds kept; pool/timeout/retry knobs frozen at `New`.

## Affected
- `simpleredis/commands_exec.go` — `runOnConn`
- `simpleredis/pool.go` — `borrow` `handedOff`
- `simpleredis/resp.go` — `do` comment
- `simpleredis/pool_test.go` and/or `simpleredis/panic_safety_test.go`; new `simpleredis/yaegi_defer_test.go`
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`
- `knowledge/devdocs/std_go_simpleredis.md`

## Out of scope
- `simpleredis/BUGS.md` (PR #68 owns it)
- Merging, closing, or cherry-picking OPEN PRs #67, #68, #69
- `heldSockets`, `lostTurns`, `turnRecoverMu`, `LostTurns()`, `recoverLostTurnsLocked`, `borrowAfterPoolWait`, or any leak-detection / turn-refill machinery (closed PRs #66 and #70)
- Creating `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md` or `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`
- Writable pool/timeout/retry knobs; third-party session imports; `unsafe` / cgo / generics; `math/rand/v2`
- Implementing product code in prepare

## Unknowns
- Probe ran against Yaegi v0.16.1. Dest already pins that version: `go.mod` / `go.sum` require `github.com/traefik/yaegi v0.16.1`; `docker-compose.yml` image `traefik:v3.7.11`; research `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/` and `ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md` record the same Traefik pin. Explore should record that as resolved rather than assumed.

## Tensions
- Dest `commands_exec.go` and `resp.go` still encode PR #29's "do not defer release" belief. This ticket overturns that with a measured Yaegi probe. Land deferred release; do not keep the #29 comment as policy.
- Dest `simpleredis/BUGS.md` section 2 still names closed `2026-09-13-simpleredis-lost-turn-recovery` as the owner of this brick. This ticket must not edit that file.
- Closed #66 / #70 recovered turns by refill without closing the panicked fd. This ticket forbids that machinery and lands the prototype defer instead.
- `origin/HEAD` is stale `origin/initial`. Dest is `master`.
