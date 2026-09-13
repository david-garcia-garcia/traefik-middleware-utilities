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

`reclaim` already owns the house shape: exported `Msg*` constants (`reclaim_dispose`), `*slog.Logger`, `logger.Error(MsgHookPanic, "key", key, ...)`. `Open` there **rejects** nil. This package copies the shape with a `simpleredis_` prefix and a different nil policy: `Config.Logger` frozen at `New`, nil means silent, no discard handler.

Happy-path `Get` on a reused socket must not build `...any` attrs. Dest `interpretedcost_test.go` and `TestAlloc*` are the regression gates. Debug events sit on miss / retry / timeout / capability paths, not on every Get.

Identity: Redis **keys** carry tenant and user ids. `Config.Pass` is a secret. The only host we may log is `Config.Host` (the Redis server address New froze). Do not reconstruct it from `net.Conn.RemoteAddr`. Do not log key names or values.

PR #69 landed on dest during implement Sync. `do` now has pre-write and post-reply `Buffered()` checks, so all 20 events fire in this apply (including `MsgSocketPoisoned` and `MsgAuthLeftover`).

## Decisions

- Mirror reclaim constants and slog key/value attrs. Put `Msg*` and the nil-safe helpers in `simpleredis/log.go`. Freeze `logger *slog.Logger` on the client in `New` from `Config.Logger`. `New` still returns only `*SimpleRedis`. Zero Config stays silent.
- Do not install `slog.DiscardHandler`. Every emit is `if sr.logger != nil`. Debug emits add `logger.Enabled(ctx, slog.LevelDebug)` **at the call site** before any attr construction (variadic `...any` or `LogAttrs`). A helper that takes `...any` still allocates at the caller; do not hide the guard in one.
- Error/Warn may allocate; they are rare. Still nil-check. No Info tier.
- Panic: keep the existing release defer first; add a second defer that recovers, logs `MsgPanic` (`panic`, `stack`, `host`), then re-panics. LIFO: recover runs first, re-panic, release still runs with `reusable==false`. Do not swallow. Extend `panic_safety_test.go` so the caller still sees the panic **and** turns/idle/`OverFrees()==0` stay as today.
- Emit at the decision site:

  | Const | Site |
  |---|---|
  | `MsgPanic` | new recover defer in `runOnConn` |
  | `MsgNoAuth` | `dial` after AUTH `do` when `errors.Is(err, errNoAuth)`; also `do` after `readReply` when the mapped error is `errNoAuth` (later NOAUTH on a live socket) |
  | `MsgNotFromNew` | `borrow` when `inUseTurns == nil` |
  | `MsgOverFree` | `freeInUseTurn` default branch after `overFrees.Add(1)` |
  | `MsgPoolExhausted` | `borrow` timer path returning `errPoolWait` |
  | `MsgShortBulk` | `readBulk` when `io.ReadFull` returns short of the announced buffer (capture `n`) |
  | `MsgBadReply` | `do` when `isDirtyProtocolError(err)` (`errIssue` / `errUnsupportedReply`) |
  | `MsgHandshakeFailed` | `dial` after AUTH or SELECT `do` when `err != nil` and not `errNoAuth` |
  | `MsgDial` | `borrow` after a successful `dial`, `reason` from that borrow (`idle_miss` vs `stale`) |
  | `MsgIdleSwept` | `borrow` when `len(stale) > 0` |
  | `MsgRetry` | `exec` when `attempt > 0` before `waitUntil` |
  | `MsgTimeout` | `ioError` OS deadline; `libraryTimeout` when it maps to `errTimeout`; `do` when `ioBound<=0` yields `errTimeout` |
  | `MsgCanceled` | `runOnConn` / `do` / `borrow` when the stop is `context.Canceled` and a socket is in play |
  | `MsgSocketClosed` | `runOnConn` when cancel forces `reusable=false`; idle-cap branch in `release` (that branch **is** the detection site — see open question) |
  | `MsgCapability` | `storeGroupWrite` when the cache becomes native or lua |
  | `MsgNoScript` | `Eval` when NOSCRIPT prefix triggers the EVAL fallback |
  | `MsgOpen` | `New` after freeze |
  | `MsgClose` | `Close` after draining idle (`idle_closed` = count closed) |

- Do not log from the generic `if !reusable { conn.close() }` arm. That arm is the sink, not the decision. Poisoned / bad-reply / panic already have their own events.
- Security: never `Pass`, never Redis key names, never values. `error` attrs are sentinel text or Redis error payloads (`WRONGPASS…`, `LOADING…`) — those are not keys. Secrets test uses a distinctive password and key and asserts the capturing handler never saw either string.
- Tests: capturing `slog.Handler` (reclaim `recHandler` shape, local to simpleredis tests). Nil-logger on every public verb. Cost: `testing.AllocsPerRun` around a Debug call site with nil logger and with a handler whose `Enabled` is false for Debug; report the numbers. Yaegi: follow `yaegi_test.go` GOPATH probe; slog is already interpreted in `reclaim/yaegi_test.go`. Do not regress `interpretedcost_test.go`.
- Spec: delta on `std_go_simpleredis_tcp-session` (Logger freeze, nil policy). Event list may fold into that family or a new leaf — propose runs FindSpecHost. Usage: update `knowledge/devdocs/std_go_simpleredis.md`.
- Implement all 20 events. `MsgSocketPoisoned` and `MsgAuthLeftover` fire at dest leftover checks in `do` (PR #69 merged during implement Sync).
- Merge last vs OPEN #69 / #72 / #73. Do not take their hunks.
- No new research folder: slog is stdlib; reclaim Yaegi already uses it; AUTH/NOSCRIPT notes already exist.

Owner levels are implemented as specified. No disagreement on Error vs Warn vs Debug. Timeout stays Debug (volume). Pool exhausted stays Warn. Over-free stays Error.

## Open questions

- Q: Who already owns the Redis server address and tenant/user identity this change must not reconstruct?
  Rank: additive asked — explore identity-ownership gate; requirement SECURITY line never log keys
  Decision: resolved — server address owner is `Config.Host` frozen by `New` (`simpleredis/simpleredis.go`). Log that field as `host`. Do not use `net.Conn.RemoteAddr`. Tenant/user identity lives on Redis key names; this change does not log keys or values. `Config.Pass` is the password owner; never log it.
  By: explore

- Q: Exact `MsgOpen` attribute set (frozen knobs besides never `Pass`)?
  Rank: additive asked — requirement Unknowns; New copies those knobs today
  Decision: assumed — log frozen values: `host`, `database`, `pool_size`, `max_idle_conns`, `pool_timeout`, `idle_timeout`, `dial_timeout`, `io_timeout`, `max_retries`, `min_retry_backoff`, `max_retry_backoff`. Never `Pass`. `database` is the SELECT index from Config, not a Redis key name.
  By: explore

- Q: Exact `reason` strings for `MsgDial` (idle miss / stale) and `MsgSocketClosed` (cancel / idle cap)?
  Rank: additive asked — requirement Unknowns; borrow already distinguishes empty-idle vs stale-sweep
  Decision: assumed — `MsgDial` `reason` is `idle_miss` when `takeIdleConn` returns no reused and no stale, `stale` when it returns no reused after a non-empty stale list. `MsgSocketClosed` `reason` is `cancel` or `idle_cap`. Snake_case to match event strings.
  By: explore

- Q: Whether `MsgNoAuth` `error` is the Redis payload or `redis:noauth`?
  Rank: additive asked — requirement Unknowns; `replyError` currently maps AUTH-class text to `errNoAuth`
  Decision: assumed — log `error` as `err.Error()` after mapping (`redis:noauth`). Do not change the caller-visible sentinel to preserve the payload. Operators already match `ErrNoAuth`; the event name is the AUTH class.
  By: explore

- Q: Whether short-bulk `read` is bytes received before the short `ReadFull`?
  Rank: additive asked — requirement Unknowns; dest `readBulk` discards `ReadFull`'s `n`
  Decision: assumed — capture `n, err := io.ReadFull(...)`; `announced` is the RESP `$` length; `read` is `n` (bytes filled into the length+2 buffer). Do not log payload bytes.
  By: explore

- Q: Whether `MsgHandshakeFailed` covers SELECT as well as AUTH when the mapped error is not `errNoAuth`?
  Rank: additive asked — requirement Desired inventory "AUTH/SELECT failed for a NON-auth reason"
  Decision: resolved — yes. `dial` after AUTH `do` and after SELECT `do`, when `err != nil && !errors.Is(err, errNoAuth)`. AUTH-class stays `MsgNoAuth` only.
  By: explore

- Q: Whether `MsgCapability` `path` is `native`/`lua` or the `groupWritePath` names?
  Rank: additive asked — requirement Unknowns; `storeGroupWrite` already records `groupWriteNative` / `groupWriteLua`
  Decision: assumed — `path` is `native` or `lua`. Log once when `storeGroupWrite` records the cache, not on every later MSetEX that reads the cache.
  By: explore

- Q: Whether non-per-command Debug (`MsgOpen` / `MsgClose`) still needs the `Enabled` guard?
  Rank: additive asked — requirement Unknowns; those paths run once per client
  Decision: assumed — nil-check only is enough for alloc; still wrap with `Enabled` so a logger at Warn+ does not build Open/Close attrs. Same shape as other Debug events. Cost test targets the per-command Debug sites (timeout/retry/dial), not New/Close.
  By: explore

- Q: `MsgSocketClosed` idle-cap is decided inside `release`; the ticket forbids logging inside `release` and forbids changing its signature. Where does that event go?
  Rank: additive asked — inventory names idle cap; dest close lives in `release` (`pool.go`); signature change is Out of scope
  Decision: assumed — dest #79 moved the idle-cap decision into `parkIdleConn`. Emit `MsgSocketClosed` reason `idle_cap` there (the detection site). `release` still has signature `(conn, reusable bool)` and does not log from the generic `!reusable` arm.
  By: implement

- Q: Caller `DeadlineExceeded` (not library budget, not `Canceled`) — `MsgTimeout` or silence?
  Rank: additive asked — requirement inventory splits I/O-or-budget timeout vs cancel; `libraryTimeout` already keeps caller deadline as `ctx.Err()`
  Decision: assumed — `MsgTimeout` only when the mapped error is `errTimeout` (OS I/O deadline or library overall budget). `MsgCanceled` only for `context.Canceled`. Caller deadline stays the caller's timer; do not emit a library timeout line for it.
  By: explore
