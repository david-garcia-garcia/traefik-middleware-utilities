## Why

SimpleRedis dial, I/O, idle age, and idle-list cap are compile-time constants (2s / 1s / 30s / 8). Planned callers (rate limiters, cache) need tens of milliseconds then fallback; a slow Redis holds every in-flight command for the full 1s I/O deadline.

## What Changes

- Add Yaegi-safe `Options` (`DialTimeout`, `IoTimeout`, `IdleTimeout` `time.Duration`; `MaxIdleConns` int) and `InitWithOptions(host, pass, database, Options)`. `Init(host, pass, database)` stays three arguments and delegates with `Options{}`.
- Zero or negative duration, and `MaxIdleConns <= 0`, mean today's constants. Do not export host/pass/database. Do not add read vs write deadlines. Do not retry `errTimeout`.
- Omit `poolSize` / `poolTimeout` from `Options` (wait-queue live-socket cap is perf-01). Idle cap still means the idle list after `release`, not live sockets.
- Keep `TestIoTimeout` on defaults. Add a fake never-reply test with configured `IoTimeout`. Prove configured `IdleTimeout` / `MaxIdleConns` on the fake. Prove `DialTimeout` with a hanging-SYN to `192.0.2.1`.
- Live proof is compiled same-package tests: `exec("BLPOP", uniqueKey, "10")` with `IoTimeout` tens of milliseconds on Redis and Dragonfly via `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. Fake-server tests are not a substitute. Do not export BLPOP. Do not stall Traefik/Pester. Probe `Config` stays `Host` only. Lua 5.1-safe. Dragonfly KEYS required if a proof uses Eval (this stall is top-level BLPOP, not Eval).
- CI `test` job sets both live addrs. Usage How to use after apply (zero = default; `Init` unchanged).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: `InitWithOptions` plus zero-value defaults for dial / I/O / idle timeout and idle-list cap; one combined `SetDeadline`; live BLPOP I/O-timeout proof on Redis and Dragonfly. Do not rewrite the concurrent-eight live-socket claim (perf-01). `std_go_simpleredis_resp-commands` unchanged.

## Impact

- `simpleredis/simpleredis.go` (`Options`, `Init`/`InitWithOptions`, `dial`, `do`, `borrow`, `release`).
- `simpleredis/simpleredis_test.go`; new `simpleredis/live_test.go`.
- `.github/workflows/ci.yml` `test` job env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`; README Tests names those vars.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` after apply.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- No compose, Pester, or `e2e/simpleredisprobe` Config change. No Yaegi `InitWithOptions` smoke required. No go-redis, miniredis, TLS, Unix sockets, functional options.
- Sibling `simpleredis-live-pool-cap` (perf-01) owns the wait-queue rewrite.
