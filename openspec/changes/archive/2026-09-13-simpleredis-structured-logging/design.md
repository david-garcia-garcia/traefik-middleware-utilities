## Context

Dest `simpleredis/` has no `log/slog` import. `reclaim/table.go` already exports `Msg*` (`reclaim_dispose` style) and takes `*slog.Logger` (required; rejects nil). This package copies that shape with a `simpleredis_` prefix and a different nil policy: optional `Config.Logger` frozen at `New`. See proposal.md for why. Proceed policies: `devstate/explore.md`. Identity: `host` is `Config.Host` frozen by `New`; do not reconstruct from `net.Conn.RemoteAddr`; do not log Redis keys.

Dest HEAD does not contain PR #69 leftover-RESP checks. OPEN #72 names `do`/`readReply` results; OPEN #73 restructures `release`. Merge last.

## Goals / Non-Goals

**Goals:**
- Optional logger, reclaim-shaped constants, 18 dest-detectable events at owner levels.
- Call-site nil + Debug `Enabled` so happy-path Get does not build `...any`.
- Panic log then re-panic with existing release defer still running.

**Non-Goals:**
- Inventing leftover-RESP detection.
- Changing `release` signature or funneling all closes through `release`.
- Discard-handler default. Info tier. Other packages' loggers.

## Decisions

1. **Injection is `Config.Logger`.** Frozen in `New` onto `sr.logger`. Alternative: per-call logger like reclaim `Open` — rejected; ticket names Config. Alternative: package-level logger — rejected; zero Config must stay silent and concurrent clients differ.

2. **No discard handler.** `if sr.logger == nil { return }` at every emit. Debug: wrap the `logger.Debug` / `LogAttrs` call with `logger.Enabled(ctx, slog.LevelDebug)` at the call site so attrs are not built when off. Alternative: a `logDebug(...any)` helper — rejected; the slice is still allocated at the caller.

3. **Constants and helpers in `simpleredis/log.go`.** `Msg*` plus tiny Error/Warn wrappers that still nil-check. Debug stays at the call site. Alternative: constants in `simpleredis.go` — rejected; 20 constants plus comments are a second job.

4. **Emit at the decision site** (explore table). Idle-cap `MsgSocketClosed` is the one event that fires from the existing idle-cap branch inside `release` because that is the detection site; signature stays `(conn, reusable bool)`. Do not log from the generic `!reusable` close.

5. **Panic defers.** Register `defer func() { sr.release(conn, reusable) }()` first, then `defer` recover/log/`simpleredis_panic`/re-panic. LIFO: recover runs first. `debug.Stack()` for `stack`. Alternative: swallow after log — rejected (ticket). Alternative: reclaim's log-and-continue — rejected (ticket panic section).

6. **`host` is `sr.host`.** Never `RemoteAddr`. `MsgOpen` attrs: host, database, pool_size, max_idle_conns, pool_timeout, idle_timeout, dial_timeout, io_timeout, max_retries, min_retry_backoff, max_retry_backoff. Never Pass. `MsgDial` reason `idle_miss` or `stale`. `MsgSocketClosed` reason `cancel` or `idle_cap`. `MsgCapability` path `native` or `lua` at `storeGroupWrite`. `MsgNoAuth` `error` is `err.Error()` after mapping (`redis:noauth`). Short-bulk captures `n` from `ReadFull`. Timeout only when mapped error is `errTimeout`. Canceled only for `context.Canceled`.

7. **Tests.** Local capturing `slog.Handler` (reclaim `recHandler` shape, not imported from reclaim). Secrets test uses distinctive `Pass` and key. Cost: `testing.AllocsPerRun` on Debug call sites with nil and Debug-disabled handlers. Yaegi GOPATH probe like `yaegi_test.go`; slog already interpreted in `reclaim/yaegi_test.go`.

8. **Two leftover events.** Declare constants so the names exist; do not fire them. Debt file already notes the follow-up after #69.

## Risks / Trade-offs

- [Happy-path Get regresses interpreted alloc] → Mitigation: no Debug emit on reused-socket success; call-site Enabled; measure `interpretedcost_test.go` and a new alloc test.
- [Panic recover swallows] → Mitigation: re-panic after log; test that recover in the test still sees the panic.
- [Secrets leak via error text or host] → Mitigation: never log args/keys/values/Pass; secrets test dumps all records as strings.
- [Idle-cap log inside release vs ticket wording] → Mitigation: only that branch; recorded as assumed on explore.md.
- [Conflict with #69/#72/#73] → Mitigation: merge last; do not take their hunks.

## Migration Plan

Additive public field. Zero Config unchanged. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
