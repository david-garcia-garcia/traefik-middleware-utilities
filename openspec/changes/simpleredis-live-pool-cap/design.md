## Context

Dest `borrow` dials whenever the idle list is empty; `release` closes sockets that would make idle exceed eight. `Init` is three strings. Spec `std_go_simpleredis_tcp-session` already says concurrent commands SHALL not open more than eight, but dest never waits. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Live cap 8 + wait + 1s timeout as `redis:unreachable`, Yaegi-safe stdlib only.
- Fake tests for burst dials, live ≤ 8, timeout; live Redis/Dragonfly compiled tests; Pester hold + `CLIENT LIST` on `/redis` and `/dragonfly`.

**Non-Goals:**
- Changing `Init` arity, public pool knobs, importing `go-redis`, perf-02 I/O-timeout fan-out, EVALSHA, pipelining, MSETEX.

## Decisions

1. **Semaphore is live sockets, not checked-out only.** Buffered `chan struct{}` of size 8 (or test override). Acquire before dial; keep the token while idle; release token only when the socket is closed. Alternative: token per checkout — rejected; idle plus in-use would exceed eight.

2. **Zero-value fields mean defaults.** Unexported `poolSize` / `poolTimeout` on `SimpleRedis`; 0 → 8 and 1s. Same-package tests set them. Alternative: export `Init` knobs — rejected; Out of scope and one production Init.

3. **Timeout string is `redis:unreachable`.** Alternative: `redis:pool-timeout` — extra public token; existing matchers would miss it.

4. **Wait uses `time.NewTimer` + `select`.** Stop the timer on grant. Do not `time.After` (leaks until fire under Yaegi load). Alternative: `sync.Cond` — more statements interpreted.

5. **Hold script is TIME wait, `numkeys` 0.** Probe `?hold=` runs Eval of a Lua 5.1-safe busy-wait on `TIME`/`PING` with no keys so Dragonfly undeclared-key does not fire. Pester overlaps holds, `docker compose exec` `redis-cli CLIENT LIST` on `redis` and `dragonfly`. Alternative: `DEBUG SLEEP` — Redis-only.

6. **CI live env mirrors siblings.** `SIMPLEREDIS_LIVE_REDIS=127.0.0.1:6379` and `SIMPLEREDIS_LIVE_DRAGONFLY=127.0.0.1:6380` on the existing `test` job services. Skip when unset. Alternative: Pester-only — caller required both engines; compiled live tests are the wire/pool proof without Traefik.

## Risks / Trade-offs

- [Default 8 waits under >8 Traefik concurrency] → Mitigation: that is the cap; fail `redis:unreachable` at 1s instead of stampeding Redis.
- [Hold EVAL burns CPU on the engine] → Mitigation: short hold (about 2s); Pester timeout above that.
- [CLIENT LIST counts other clients] → Mitigation: filter by Traefik source / cmd, or count delta vs baseline before the burst.
- [Yaegi `select` + timer] → Mitigation: stdlib only; existing session already uses `time`.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
