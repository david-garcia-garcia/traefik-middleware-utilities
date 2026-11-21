# Requirement
IssueKey: 2026-09-13-simpleredis-structured-logging

## Problem
`simpleredis` has no structured logs. Pool poison, AUTH failure, over-free, panic, handshake failure, and retry all happen silently except as returned errors. Operators cannot tell a defended peer glitch from a broken invariant. Dest already has `reclaim`'s `*slog.Logger` + `reclaim_*` message constants; this package has neither.

## Current (code)
- `simpleredis/config.go` `Config` — Host, Pass, Database, pool/timeout/retry knobs. No `Logger`. Comment: zero Config uses package defaults. `New` returns `*SimpleRedis` only.
- `simpleredis/simpleredis.go` `New` — copies those knobs; does not copy a logger. `Close` drains idle sockets with no log. `OverFrees()` reads `overFrees`.
- `simpleredis/commands_exec.go` `runOnConn` — `reusable := false`; `defer release(conn, reusable)`; then `do`. No recover. Panic still reaches the caller; PR #71 (`c14cbec`) already returns the turn via that defer (`simpleredis/panic_safety_test.go` recovers in the test, not inside `runOnConn`).
- `simpleredis/pool.go` `freeInUseTurn` — full semaphore → `overFrees.Add(1)` and drop. No log.
- `simpleredis/pool.go` `borrow` — `inUseTurns == nil` → `errNotFromNew`. Waiter past `PoolTimeout` → `errPoolWait`. Idle miss → `dial`. `takeIdleConn` returns stale list; borrow closes them with no log.
- `simpleredis/pool.go` `dial` — AUTH then SELECT via `do`. AUTH-class errors come back as `errNoAuth` from `replyError`; LOADING/max-clients stay `errors.New(text)` with `handshakeFailed true`. No log. Handshake leftover: dest `do` does not inspect `Buffered()`.
- `simpleredis/pool.go` `release` — `reusable false` closes; idle-cap/closed also closes. Signature `(conn, reusable bool)` only. No reason.
- `simpleredis/resp.go` `replyError` — prefixes `NOAUTH` / `WRONGPASS` / `NOPERM` / `ERR Client sent AUTH` → `errNoAuth`. Other `-` payloads stay the Redis text.
- `simpleredis/resp.go` `readReply` / `readBulk` — malformed/unsupported → `errIssue` / `errUnsupportedReply`. Truncated bulk (`io.ReadFull` short of announced length) returns the IO error; `do` maps it via `ioError` to `redis:unreachable`. No leftover-bytes check (`Buffered()` appears only in `simpleredis/BUGS.md`, not in dest `do`).
- `simpleredis/resp.go` `ioError` / `commands_exec.go` `libraryTimeout` — OS deadline → `errTimeout`; library-owned ctx deadline → `errTimeout`; caller cancel stays `ctx.Err()`.
- `simpleredis/commands_eval.go` `Eval` — `strings.HasPrefix(err.Error(), "NOSCRIPT")` then a second `exec` of EVAL. No log.
- `simpleredis/commands_msetex.go` `msetex` / `storeGroupWrite` — caches native vs Lua after unknown-command. No log.
- `reclaim/table.go` — `*slog.Logger`, exported `Msg*` (`reclaim_dispose` style), `logger.Error(MsgHookPanic, "key", key, ...)`. `Open` **rejects** nil logger. Hook panic is logged and not re-panicked (`dispose` / `runHook`).
- `reclaim/table_test.go` `recHandler` — capturing `slog.Handler` for msg/level/attrs. Yaegi already uses slog in reclaim tests.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — New copies host/password/database/pool/I/O/retry; no Logger; stdlib-only imports (`log/slog` is stdlib).
- `knowledge/devdocs/std_go_simpleredis.md` — how to New/Get/Eval/MSetEX. No Logger, no event names.
- `simpleredis/interpretedcost_test.go` — Yaegi Get/encode benches. `simpleredis/bench_test.go` `TestAlloc*` — compiled alloc ceilings.
- `knowledge/research/ext_redis_auth/notes.md` — WRONGPASS / nopass AUTH text / NOPERM-not-from-AUTH.
- `knowledge/research/ext_redis_evalsha/notes.md` — NOSCRIPT prefix.
- Dest HEAD `c14cbec10118f9b27d92ec507394e804e034d7ec` does not contain PR #69. OPEN: #69 leftover-RESP destroy, #72 golangci named results on `do`/`readReply`, #73 `release` restructure.

## Desired
1. Public `Config.Logger *slog.Logger`, frozen at `New` like other knobs. `New` never errors. nil logger = silent. Do not install a discard handler.
2. Exported `Msg*` constants with `simpleredis_` event strings, slog key/value attrs, reclaim shape. Call-site nil (and Debug: `Enabled` or `LogAttrs`) so Debug paths allocate nothing when off.
3. Emit the owner-approved inventory at the decision site, not inside `release`. Never log `Pass`, Redis keys, or values — counts, reasons, byte lengths only.
4. Error (4): panic (recover, log, re-panic), noauth, not-from-New, over-free. Warn (6): socket poisoned, auth leftover, pool exhausted, short bulk, bad reply, handshake failed (non-auth). Debug (10): dial, idle swept, retry, timeout, canceled, socket closed, capability, noscript, open, close. No Info.
5. Panic: register existing release defer first, then recover/log/re-panic defer. After a panic: turn returned, socket closed, `OverFrees()==0`, panic still reaches the caller.
6. Dest lacks leftover-RESP detection (#69 still OPEN). Implement the other 18 events; do not invent leftover detection. Merge last relative to #69 / #72 / #73.
7. Tests: capturing handler per event; nil-logger no panic on every command path; Debug alloc=0 when nil or Debug off (report numbers); secrets test (distinctive password + key never appear at most verbose); Yaegi logging test (`yaegi_test.go` pattern); `interpretedcost_test.go` must not regress. Local suite + `go vet` + measured CI.

## Affected
- `simpleredis/config.go`, `simpleredis/simpleredis.go`, `simpleredis/resp.go`, `simpleredis/pool.go`, `simpleredis/commands_exec.go`, `simpleredis/commands_eval.go`, `simpleredis/commands_msetex.go`
- New/extended tests (capturing handler, nil logger, secrets, panic re-raise, cost, Yaegi)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (and propose: event/level leaf if librarian splits)
- `knowledge/devdocs/std_go_simpleredis.md`

## Out of scope
- Changing `release(conn, reusable bool)` signature or logging from inside `release`.
- Defaulting nil to `slog.DiscardHandler` / a no-op handler.
- Inventing leftover-RESP / pre-write-reader detection while #69 is not on dest.
- Logging `Config.Pass`, Redis key names, or values.
- An Info tier, or redesigning owner-approved levels.
- Swallowing panics after log.
- Other packages' loggers (windowcounter, tokenbucket). Mirroring reclaim's *shape* only.
- Local `go test -race` (no C toolchain; CI race job).

## Unknowns
- Exact `MsgOpen` attribute set (frozen knobs besides never `Pass`).
- Exact `reason` strings for `MsgDial` (idle miss / stale) and `MsgSocketClosed` (cancel / idle cap).
- Whether `MsgNoAuth` `error` is the Redis payload or `redis:noauth`.
- Whether short-bulk `read` is bytes received before the short `ReadFull`.
- Whether `MsgHandshakeFailed` covers SELECT as well as AUTH when the mapped error is not `errNoAuth`.
- Whether `MsgCapability` `path` is `native`/`lua` or the `groupWritePath` names.
- Whether non-per-command Debug (`MsgOpen` / `MsgClose`) still needs the `Enabled` guard.

## Tensions
- Ticket: follow reclaim convention exactly. Dest `reclaim/table.go` `Open` rejects nil logger; this ticket requires accepting nil and not discarding. Injection is `Config` (ticket), not per-call (reclaim). Shape (constants, slog attrs, `simpleredis_` prefix) is the honouring; nil policy is the ticket.
- Ticket: never log Redis keys. Reclaim logs table `key`. SECURITY wins; do not copy that attribute onto Redis names.
- Ticket: recover, log, **re-panic**. Reclaim logs hook panics and continues. Ticket panic section wins; reclaim is not the panic owner here.
- Ticket inventory is 20 events. Dest has no leftover-bytes site (#69 OPEN, not in `c14cbec`). Ticket says implement 18 and follow-up the two. Do not invent detection.
- Merge last: this change edits the same files as OPEN #72 (named results on `do`/`readReply`) and #73 (`release` in `pool.go`).
- Owner: timeout is Debug (volume); pool exhausted is Warn (transient); over-free is Error (our invariant). Implement as specified; do not raise the level of timeout.
