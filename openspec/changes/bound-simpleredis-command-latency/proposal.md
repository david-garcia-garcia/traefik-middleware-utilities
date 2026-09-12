## Why

On master, a stock SimpleRedis `Get` against an unreachable Redis spends about 8s (four dials × 2s `DialTimeout` plus backoff). No public verb takes `context.Context`, so a cancelled Traefik request keeps working. AUTH/SELECT each start a fresh I/O clock after TCP connects.

## What Changes

- Shorter zero-Config defaults: `DialTimeout` 200ms, `IOTimeout` 100ms, one extra retry (`MaxRetries` 0 at `New` → 1 extra). `-1` still means no extra retries. Explicit `MaxRetries: 3` still means three extra.
- One overall deadline per command, derived as `(maxRetries+1)*(DialTimeout+IOTimeout)`, enforced across attempts. Remaining time is the bound for dial and for AUTH/SELECT/command so handshake steps cannot each add a full `IOTimeout`.
- Public `*Context` twins (`GetContext`, `EvalContext`, …). Today’s signatures wrap `context.Background()`. Cancel returns `ctx.Err()`; derived-budget expiry returns `redis:timeout`. Cancel closes the socket and frees the pool turn. No `net.Error` assert.
- Compiled proof: black-hole `203.0.113.1:6379` elapsed under the derived budget; accept-then-stall fake with password and database; cancel mid-command.
- **Not this change:** circuit breaker (see `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md`). Probe, windowcounter, and tokenbucket stay on unadorned verbs.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: shorter zero-Config dial/I/O/retry defaults; overall command deadline; remaining budget on dial handshake; context cancel of the session path.
- `std_go_simpleredis_resp-commands`: every public verb gains a `*Context` twin; unadorned methods keep today’s meaning via `context.Background()`.

## Impact

- `simpleredis/config.go`, `commands_exec.go`, `pool.go`, `resp.go`, `commands.go`, `commands_eval.go`, `commands_msetex.go`, matching `*_test.go`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- `knowledge/devdocs/std_go_simpleredis.md` (worst-case product next to the knobs).
- Yaegi stdlib `context` only. No go-redis, miniredis, TLS, Unix sockets. `e2e/simpleredisprobe` unadorned verbs stay.
