# Add structured logging to simpleredis

Add structured logging to the `simpleredis` package so production misbehavior is diagnosable instead of silent. Deliver ONE pull request. PR host is GitHub. Base on `origin/master` at the time you start.

The event inventory below was reviewed and approved by the repo owner. Implement it as specified. If you believe an event or level is wrong, implement what is specified and raise the disagreement in your report — do not silently deviate.

## Design, already decided — do not redesign these

1. **Follow the `reclaim` package's existing convention exactly.** Read `reclaim/table.go` first. It uses `*slog.Logger`, exported message-name constants (`MsgHookPanic`, `MsgDispose`, `MsgBind`, ... rendering as `reclaim_dispose` style strings), and structured key/value attributes: `logger.Error(MsgHookPanic, "key", key, "hook", "close", "panic", recovered)`. Mirror that shape with a `simpleredis_` prefix. `log/slog` is Go 1.21 stdlib and `reclaim`'s Yaegi tests already prove it survives the interpreter.

2. **Injection point:** a new `Logger *slog.Logger` field on `simpleredis.Config`, frozen onto the client at `New` like every other Config knob. This is PUBLIC API, so it needs an openspec spec update and a `knowledge/devdocs/std_go_simpleredis.md` update.

3. **The logger MUST be optional.** `New(Config)` returns no error and cannot reject a nil logger, and `Config` is documented as zero-value-friendly ("A zero Config uses the package defaults"). nil means completely silent.

4. **Do NOT default nil to a discard handler.** Guard at the call site instead. `simpleredis/interpretedcost_test.go` measures interpreted overhead, and building variadic `...any` attributes on every `Get` would regress it even when the output is discarded. For the per-command Debug events, guard with a nil check AND `logger.Enabled(ctx, slog.LevelDebug)` (or use `LogAttrs`) so nothing is allocated when Debug is off. Measure this, do not assume it.

5. **SECURITY, non-negotiable.** Never log `Config.Pass`. Never log Redis key names or values, at any level — keys carry tenant and user identifiers. `gosec` is enabled and the repo has a security review axis. Log counts, reasons, and byte lengths instead of content.

6. **Log at the site that makes the decision, not inside `release`.** `release(conn, reusable bool)` cannot carry a reason, and changing its signature is out of scope. Each event should be emitted where the condition is actually detected.

## The inventory

Level rule the owner set: **Error means an operator must act, or the condition is unrecoverable.** Anything the library already defends against is Warn or lower. There is NO Info tier in this package.

### Error (4)

| Constant | Event string | Fires when | Attributes |
|---|---|---|---|
| `MsgPanic` | `simpleredis_panic` | recovered in `runOnConn`, logged, then re-panicked | `panic`, `stack`, `host` |
| `MsgNoAuth` | `simpleredis_noauth` | AUTH rejected: `WRONGPASS` / `NOAUTH` / `NOPERM` / `ERR Client sent AUTH` | `host`, `error` |
| `MsgNotFromNew` | `simpleredis_not_from_new` | a command runs on a client built as `&SimpleRedis{}` instead of via `New` | `host` |
| `MsgOverFree` | `simpleredis_over_free` | `OverFrees` increments in `freeInUseTurn` | `over_frees`, `turns`, `cap` |

### Warn (6)

| Constant | Event string | Fires when | Attributes |
|---|---|---|---|
| `MsgSocketPoisoned` | `simpleredis_socket_poisoned` | leftover RESP remains after a complete reply; socket destroyed | `buffered`, `host` |
| `MsgAuthLeftover` | `simpleredis_auth_leftover` | pre-write reader not empty (handshake path) | `buffered`, `host` |
| `MsgPoolExhausted` | `simpleredis_pool_exhausted` | a waiter exceeds `PoolTimeout` | `pool_size`, `wait`, `host` |
| `MsgShortBulk` | `simpleredis_short_bulk` | peer announced more bulk payload than it wrote | `announced`, `read`, `host` |
| `MsgBadReply` | `simpleredis_bad_reply` | malformed or unsupported RESP (`redis:issue?`, `redis:unsupported-reply`) | `error`, `host` |
| `MsgHandshakeFailed` | `simpleredis_handshake_failed` | AUTH/SELECT failed for a NON-auth reason (`LOADING`, max clients) | `error`, `host` |

### Debug (10)

| Constant | Event string | Fires when | Attributes |
|---|---|---|---|
| `MsgDial` | `simpleredis_dial` | a new socket is opened | `reason` (idle miss / stale), `host` |
| `MsgIdleSwept` | `simpleredis_idle_swept` | stale idle sockets closed on borrow | `swept`, `remaining` |
| `MsgRetry` | `simpleredis_retry` | a retry attempt is scheduled | `attempt`, `backoff`, `error` |
| `MsgTimeout` | `simpleredis_timeout` | I/O deadline or overall command budget elapsed | `error`, `host` |
| `MsgCanceled` | `simpleredis_canceled` | caller context cancelled mid-command | `host` |
| `MsgSocketClosed` | `simpleredis_socket_closed` | socket destroyed for a benign reason (cancel, idle cap reached) | `reason` |
| `MsgCapability` | `simpleredis_capability` | `MSetEX` resolves native vs Lua fallback | `path` |
| `MsgNoScript` | `simpleredis_noscript` | `EVAL` NOSCRIPT triggers a script reload | none beyond `host` |
| `MsgOpen` | `simpleredis_open` | `New` | frozen config values (NEVER `Pass`) |
| `MsgClose` | `simpleredis_close` | `Close` | `idle_closed` |

Notes the owner made explicitly:
- `simpleredis_timeout` is DEBUG on purpose despite being interesting, because under load it can fire thousands of times per second and would drown everything else.
- `simpleredis_pool_exhausted` is WARN not Error: it is transient load pressure, even though it hints at raising `PoolSize`.
- `simpleredis_over_free` is ERROR even though `freeInUseTurn` drops the extra token and continues, because unlike leftover RESP it is not a peer behavior we defend against — it is our own invariant broken, with no legitimate cause.

## The panic event needs care — read this twice

PR #71 added `runOnConn` in `simpleredis/commands_exec.go`, which does `reusable := false` then `defer func() { sr.release(conn, reusable) }()` before calling `sr.do`. You must add panic logging WITHOUT changing behavior: recover, log at Error, then **re-panic**. Never swallow the panic.

Defer ordering is load-bearing and is the easiest thing to get wrong. Register the existing release defer FIRST, then the recover/log/re-panic defer, so that (defers being LIFO) the recover defer runs first, logs, re-panics, and the release defer then still runs during the propagating panic with `reusable` still `false` — destroying the socket and returning the in-use turn. Prove with a test that after a recovered panic the turn is returned, the socket is closed, `OverFrees()` is 0, AND the panic still reaches the caller.

## Sequencing and conflicts

This PR touches `config.go`, `simpleredis.go`, `resp.go`, `pool.go`, `commands_exec.go`, `commands_eval.go`, and `commands_msetex.go`, so it conflicts with everything currently in flight and should merge LAST. Two of its events, `simpleredis_socket_poisoned` and `simpleredis_auth_leftover`, only have a detection site once PR #69 (`2026-09-13-simpleredis-desync-boundary-check`) merges. If #69 is not on `master` when you run, implement the other 18 events and record the two missing ones as a follow-up rather than inventing detection logic. Also in flight: `2026-09-13-golangci-lint-harden` (hardens `.golangci.yml`, adds named result parameters to `do` and `readReply`) and `2026-09-13-simpleredis-idle-mutex-defer` (restructures `release` in `pool.go`).

## Tests required

- A capturing `slog.Handler` in tests; assert each event fires on its path with the right level and attributes.
- A nil-logger test: every command path works and nothing panics with `Config.Logger` unset.
- A cost test: confirm no allocation for the Debug events when the logger is nil or Debug is disabled. Report measured numbers.
- **A secrets test**: run commands with a distinctive password AND a distinctive key name, capture ALL log output at the most verbose level, and assert neither string appears anywhere. This is the guard for requirement 5.
- A Yaegi test proving logging works interpreted, following the existing `simpleredis/yaegi_test.go` patterns.
- `simpleredis/interpretedcost_test.go` must not regress.
- Full local suite plus `go vet`, then measured CI green.
