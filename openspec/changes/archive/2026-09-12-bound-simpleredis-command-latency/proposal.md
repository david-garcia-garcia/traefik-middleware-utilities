## Why

On master, a stock SimpleRedis `Get` against an unreachable Redis spends about 8s (four dials × 2s `DialTimeout` plus backoff). No public verb takes `context.Context`, so a cancelled Traefik request keeps working. AUTH/SELECT each start a fresh I/O clock after TCP connects.

## What Changes

- Shorter zero-Config defaults: `DialTimeout` 200ms, `IOTimeout` 100ms, one extra retry (`MaxRetries` 0 at `New` → 1 extra). `-1` still means no extra retries. Explicit `MaxRetries: 3` still means three extra.
- One overall deadline per command, derived as `(maxRetries+1)*(DialTimeout+IOTimeout)`, enforced across attempts. Remaining time is the bound for dial and for AUTH/SELECT/command so handshake steps cannot each add a full `IOTimeout`.
- Every public verb takes `context.Context` as its first argument (`Get(ctx, name)`, `Eval(ctx, …)`). A caller with no deadline passes `context.Background()` at the call site. Cancel returns `ctx.Err()`; derived-budget expiry returns `redis:timeout`. Cancel closes the socket and frees the pool turn. No `net.Error` assert.
- Compiled proof: black-hole `203.0.113.1:6379` elapsed under the derived budget; accept-then-stall fake with password and database; cancel mid-command.
- **Not this change:** circuit breaker (see `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md`). Windowcounter `Take`/`Peek`/`Allow` and tokenbucket `Allow` take a context and pass it into SimpleRedis. The flush ticker still uses `context.Background()`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: shorter zero-Config dial/I/O/retry defaults; overall command deadline; remaining budget on dial handshake; context cancel of the session path.
- `std_go_simpleredis_resp-commands`: every public verb takes `context.Context` first; callers pass `context.Background()` when they have no deadline.
- `std_go_windowcounter_sliding-take`: `Take`/`Peek`/`Allow` take `context.Context` first.
- `std_go_tokenbucket_allow`: `Allow` takes `context.Context` first on Memory and Redis.

## Impact

- `simpleredis/config.go`, `commands_exec.go`, `pool.go`, `resp.go`, `commands.go`, `commands_eval.go`, `commands_msetex.go`, matching `*_test.go`.
- `windowcounter/limiter.go`, `tokenbucket/redis.go`, `tokenbucket/memory.go`, matching tests.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`, `openspec/specs/std_go_windowcounter_sliding-take/spec.md`, `openspec/specs/std_go_tokenbucket_allow/spec.md` after archive.
- `knowledge/devdocs/std_go_simpleredis.md`, `std_go_windowcounter.md`, `std_go_tokenbucket.md`.
- Yaegi stdlib `context` only. No go-redis, miniredis, TLS, Unix sockets. `e2e/simpleredisprobe` passes `req.Context()`.
