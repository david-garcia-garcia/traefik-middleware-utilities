## Context

Dest `borrow` dials whenever the idle list is empty; `release` closes sockets that would make idle exceed eight. `Init` is three strings. Spec `std_go_simpleredis_tcp-session` already says concurrent commands SHALL not open more than eight, but dest never waits. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Live cap 8 + wait + 200ms timeout as `redis:unreachable`, Yaegi-safe stdlib only.
- Fake tests for burst dials, live ≤ 8, timeout; live Redis/Dragonfly compiled tests; Pester hold + `/proc/net/tcp` ESTABLISHED on `/redis` and `/dragonfly`.

**Non-Goals:**
- Changing `Init` arity, public pool knobs, importing `go-redis`, perf-02 I/O-timeout fan-out, EVALSHA, pipelining, MSETEX.

## Decisions

1. **Semaphore is in-use turns, not idle-plus-in-use.** Buffered `chan struct{}` of size 8 (or test override). `borrow` waits for a turn, then pops idle or dials. `release` returns the turn so waiters wake and reuse idle. Alternative: token stays with the socket including idle — rejected; waiters already past the idle check would block until `poolTimeout` while eight sit idle.

2. **Zero-value fields mean defaults.** Unexported `poolSize` / `poolTimeout` on `SimpleRedis`; 0 → 8 and 200ms. Same-package tests set them. Alternative: export `Init` knobs — rejected; Out of scope and one production Init. Alternative: 1s pool wait matching `ioTimeout` — rejected; a live EVAL hold cannot outlast a 1s I/O deadline, so Pester could not observe the waiter error.

3. **Timeout string is `redis:unreachable`.** Alternative: `redis:pool-timeout` — extra public token; existing matchers would miss it.

4. **Wait uses `time.NewTimer` + `select`.** Stop the timer on grant. Do not `time.After` (leaks until fire under Yaegi load). Alternative: `sync.Cond` — more statements interpreted.

5. **Hold script is TIME wait, `numkeys` 0.** Probe `?hold=` runs Eval of a Lua 5.1-safe busy-wait on `TIME` with no keys so Dragonfly undeclared-key does not fire. Redis CLIENT LIST is blocked during that script; Pester counts ESTABLISHED `/proc/net/tcp` sockets on port 6379 in the engine netns (Redis has `cat`; Dragonfly is distroless so a `redis:7-alpine` sidecar shares that netns). Compiled live tests count the runner's `/proc/net/tcp` outbound to 6379/6380. Hold is 500ms so eight serial Redis scripts still overlap a 200ms waiter before `ioTimeout`. Alternative: `DEBUG SLEEP` — Redis-only.

6. **CI live env mirrors siblings.** `SIMPLEREDIS_LIVE_REDIS=127.0.0.1:6379` and `SIMPLEREDIS_LIVE_DRAGONFLY=127.0.0.1:6380` on the existing `test` job services. Skip when unset. Alternative: Pester-only — caller required both engines; compiled live tests are the wire/pool proof without Traefik.

## Risks / Trade-offs

- [Default 8 waits under >8 Traefik concurrency] → Mitigation: that is the cap; fail `redis:unreachable` at 1s instead of stampeding Redis.
- [Hold EVAL burns CPU on the engine] → Mitigation: 500ms hold, shorter than `ioTimeout`; Pester HTTP timeout 15s.
- [CLIENT LIST counts other clients] → Mitigation: filter by Traefik source / cmd, or count delta vs baseline before the burst.
- [Yaegi `select` + timer] → Mitigation: stdlib only; existing session already uses `time`.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
