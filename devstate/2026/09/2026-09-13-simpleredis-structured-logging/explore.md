# Explore
IssueKey: 2026-09-13-simpleredis-structured-logging

## Concepts

```
  dest command (silent)
  ─────────────────────────────────────────────
  New ──► exec ──► borrow ──► runOnConn ──► do ──► release
            │         │            │         │         │
            │         │            │         │         └─ idle-cap / dirty close
            │         │            │         └─ AUTH map, short bulk, bad RESP, I/O deadline
            │         │            └─ panic: release defer only (no log)
            │         └─ not-from-New, pool wait, idle sweep, dial, AUTH/SELECT
            └─ retry, library timeout, caller cancel
```

`reclaim` already owns exported `Msg*` constants and rejects a nil logger. This package does **not** copy that catalog. `Config.Logger` is frozen at `New`; nil is replaced with a discard logger (`slog.NewTextHandler` on `io.Discard`) so call sites never nil-check. Emits are inline `simpleredis_*` strings at the decision site. No `log.go`, no message constants, no extra types to capture log attributes.

Happy-path `Get` on a reused socket must not build `...any` attrs. Dest `interpretedcost_test.go` and `TestAlloc*` are the regression gates. Debug events sit on miss / retry / timeout / capability paths, not on every Get.

Identity: Redis **keys** carry tenant and user ids. `Config.Pass` is a secret. The only host we may log is `Config.Host` (the Redis server address New froze). Do not reconstruct it from `net.Conn.RemoteAddr`. Do not log key names or values.

PR #69 landed on dest during implement Sync. `do` now has pre-write and post-reply `Buffered()` checks, so all 20 events fire in this apply (including `MsgSocketPoisoned` and `MsgAuthLeftover`).

## Decisions

- Human reshape: no `log.go`, no `Msg*` constants, no nil-safe helpers. Freeze `logger *slog.Logger` on the client in `New`. Nil `Config.Logger` becomes a discard logger. Call sites use `sr.logger.Error/Warn/Debug("simpleredis_…")` with values already in scope. `New` still returns only `*SimpleRedis`.
- Do not wrap control flow to capture extra log attributes (dial reason, short-bulk announced/read, idle-cap vs cancel close reason, Open knobs, Close idle count). Happy-path Get still does not log.
- No Info tier. Never log `Pass`, Redis keys, or values.
- Panic: keep the existing release defer first; add a second defer that recovers, logs `simpleredis_panic`, then re-panics. Do not swallow. Turns/idle/`OverFrees()==0` stay as today.
- Emit inline at the decision site: panic, noauth, over-free, pool exhausted, short bulk, bad reply, handshake failed, leftover (poisoned / auth leftover), dial, idle swept, retry, timeout, canceled, capability, noscript, open. Do not emit not-from-new, socket-closed, or close.
- Tests: capturing `slog.Handler` local to simpleredis tests. Nil-logger (discard) on every public verb. Yaegi GOPATH probe observes `simpleredis_open`. Do not regress `interpretedcost_test.go`.
- Spec: `std_go_simpleredis_tcp-session` (Logger freeze, discard-at-New). Event list: `std_go_simpleredis_slog-events`. Usage: `knowledge/devdocs/std_go_simpleredis.md`.

## Open questions

- Q: Who already owns the Redis server address and tenant/user identity this change must not reconstruct?
  Rank: additive asked — explore identity-ownership gate; requirement SECURITY line never log keys
  Decision: resolved — server address owner is `Config.Host` frozen by `New` (`simpleredis/simpleredis.go`). Log that field as `host`. Do not use `net.Conn.RemoteAddr`. Tenant/user identity lives on Redis key names; this change does not log keys or values. `Config.Pass` is the password owner; never log it.
  By: explore

- Q: Exact `MsgOpen` attribute set (frozen knobs besides never `Pass`)?
  Rank: additive asked — requirement Unknowns; New copies those knobs today
  Decision: resolved — `simpleredis_open` logs `host` only. Do not dump frozen knobs. Never `Pass`.
  By: implement

- Q: Exact `reason` strings for `MsgDial` (idle miss / stale) and `MsgSocketClosed` (cancel / idle cap)?
  Rank: additive asked — requirement Unknowns; borrow already distinguishes empty-idle vs stale-sweep
  Decision: resolved — no reason attrs. Dial is `simpleredis_dial` after a successful dial. Do not emit `simpleredis_socket_closed`.
  By: implement

- Q: `origin/master` PR #87 added a second reason to dial (`skipIdle`: this command already failed I/O on a socket it took from the unused list). Does `simpleredis_dial` still carry no reason?
  Rank: additive asked — dest sync reopened the resolved "no reason attrs" row; the new path is the one this feature exists to make visible
  Decision: assumed — `simpleredis_dial` carries `reason` `idle_miss` or `skip_idle`. Without it a peer restart that poisons every parked socket and ordinary pool pressure produce the same line, and separating them is the diagnosis PR #87 exists to enable. `simpleredis_socket_closed` stays unemitted.
  By: mergeconflictresolve

- Q: Whether `MsgNoAuth` `error` is the Redis payload or `redis:noauth`?
  Rank: additive asked — requirement Unknowns; `replyError` currently maps AUTH-class text to `errNoAuth`
  Decision: resolved — inline `simpleredis_noauth` with no required `error` attr. Caller-visible sentinel stays `redis:noauth`.
  By: implement

- Q: Whether short-bulk `read` is bytes received before the short `ReadFull`?
  Rank: additive asked — requirement Unknowns; dest `readBulk` discards `ReadFull`'s `n`
  Decision: resolved — do not capture `n`. Dest `readBulk` still returns the I/O error. Warn is `simpleredis_short_bulk` when that error is EOF / unexpected EOF.
  By: implement

- Q: Whether `MsgHandshakeFailed` covers SELECT as well as AUTH when the mapped error is not `errNoAuth`?
  Rank: additive asked — requirement Desired inventory "AUTH/SELECT failed for a NON-auth reason"
  Decision: resolved — yes. `dial` after AUTH `do` and after SELECT `do`, when `err != errNoAuth`. AUTH-class stays `simpleredis_noauth` only.
  By: explore

- Q: Whether `MsgCapability` `path` is `native`/`lua` or the `groupWritePath` names?
  Rank: additive asked — requirement Unknowns; `storeGroupWrite` already records `groupWriteNative` / `groupWriteLua`
  Decision: resolved — no `path` attr. Log `simpleredis_capability` when `storeGroupWrite` records the cache.
  By: implement

- Q: Whether non-per-command Debug (`MsgOpen` / `MsgClose`) still needs the `Enabled` guard?
  Rank: additive asked — requirement Unknowns; those paths run once per client
  Decision: resolved — no Enabled guard and no Close event. Open is one Debug line at New. Discard logger at New when Logger is nil.
  By: implement

- Q: `MsgSocketClosed` idle-cap is decided inside `release`; the ticket forbids logging inside `release` and forbids changing its signature. Where does that event go?
  Rank: additive asked — inventory names idle cap; dest close lives in `release` (`pool.go`); signature change is Out of scope
  Decision: resolved — do not emit `simpleredis_socket_closed`. Leave `parkIdleConn` as dest.
  By: implement

- Q: Caller `DeadlineExceeded` (not library budget, not `Canceled`) — `MsgTimeout` or silence?
  Rank: additive asked — requirement inventory splits I/O-or-budget timeout vs cancel; `libraryTimeout` already keeps caller deadline as `ctx.Err()`
  Decision: resolved — `simpleredis_timeout` only when the mapped error is `errTimeout` at the I/O site. `simpleredis_canceled` only for `context.Canceled`. Caller deadline stays silent. Do not wrap `libraryTimeout` just to log.
  By: implement

- Q: `origin/master` PR #84 moved the `redis:timeout` decision out of `do` (`ioOrContext` now reports the socket deadline as a context deadline and `libraryTimeout` classifies it). Where does `simpleredis_timeout` emit?
  Rank: additive asked — dest sync made the resolved "at the I/O site" answer unreachable on the exec path
  Decision: assumed — emit in `libraryTimeout`, which became a method on `*SimpleRedis`; `do` no longer emits. Measured: with the `exec` emit disabled, `TestLogTimeout` fails and the captured dump holds only `simpleredis_open` and `simpleredis_dial`. Caller deadline still stays silent.
  By: mergeconflictresolve
