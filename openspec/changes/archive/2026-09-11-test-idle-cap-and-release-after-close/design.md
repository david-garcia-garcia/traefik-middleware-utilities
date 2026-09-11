## Context

Dest `simpleredis/` already caps idle at `maxIdleConns = 8` in `release` and closes in-flight sockets when `sr.closed` is set. Cover profile count is 0 on those close paths and on `borrow`'s second `closed` check. `startFakeRedis` accepts immediately; `startSlowRedis` / `bench_test.go` are not on dest. Probe `e2e/simpleredisprobe` is one `SimpleRedis` per plugin with sequential verbs per request (existing EVAL is Lua 5.1-safe and KEYS-declared). Compose Redis and Dragonfly do not publish 6379 to the host. See proposal.md for why. Usage (`knowledge/devdocs/std_go_simpleredis.md`) already states idle eight after release and in-flight uncapped. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Same-file hold fake so overlap is deterministic.
- Unit proofs of idle cap, in-flight Close, and the second `closed` check.
- Pester idle-cap smoke via `CLIENT LIST` on both engines.

**Non-Goals:**
- Total connection cap or wait queue (perf-01).
- Changing `maxIdleConns`, `idleTimeout`, `ioTimeout`, or `dialTimeout`.
- Probe idle header, `IdleCount` export, host port publish, new EVAL script, `bench_test.go`.
- Documenting `:236-238` as untested (the hook is in scope).

## Decisions

1. **Hold fake in `simpleredis_test.go`.** Accept or reply gated on a channel, modeled on `TestTimeoutOnReusedConnIsNotRetried`'s delayed listener. Start 16 Gets, release the hold after ≥16 accepts, then assert `len(idle) <= 8` and live fake sockets equal `len(idle)` (excess closed, not leaked). Track currently-open accepted conns (dest `fakeRedis.conns` only increments). Alternative: `bench_test.go` / `startSlowRedis` — not on dest; do not add that file.

2. **Repair existing tests, do not delete Close-then-Get.** Raise `TestConcurrentCommandsStayWithinPool` above eight goroutines and assert `len(idle) <= 8` (the old eight-accept check cannot fail). Keep Close-then-Get `redis:unreachable` and no-redial. Move the idle-empty-after-release proof to a new in-flight-Close test that holds a command, calls `Close`, then asserts idle empty and the socket closed.

3. **Nil-checked same-package hook for the second `closed` check.** After idle-scan unlock and before the second `closed` read, call a package-level func if non-nil. The test sets it to `Close`. Nil in production is not a pool-behavior change. Alternative: document `:236-238` untested — rejected; Desired allows the hook and explore took it.

4. **Pester `CLIENT LIST`, no probe change.** 16 parallel `Invoke-WebRequest` at existing `/redis` then the same at `/dragonfly`. After they finish, `docker compose exec redis redis-cli CLIENT LIST` and `docker compose exec redis redis-cli -h dragonfly CLIENT LIST` (bare, no `TYPE`/`ID`; Dragonfly rejects those filters). Count LF `property=value` lines minus the listing client; remaining ≤ 8. Existing ServeHTTP EVAL stays. Alternative: idle-count header or `IdleCount` export — rejected; do not grow the probe or public API for a smoke check.

## Risks / Trade-offs

- [16 parallel HTTP requests may not overlap as tightly as the unit hold fake] → Mitigation: unit tests own the deterministic proof; live is smoke against both engines.
- [CLIENT LIST includes the listing `redis-cli`] → Mitigation: subtract that one line; use the no-argument form (Dragonfly `TYPE`/`ID` is a syntax error).
- [Hook is a production symbol tests-only] → Mitigation: nil-checked; name it for test (`afterIdleScanForTest` or equivalent with test in the name); do not export.
- [Yaegi GOPATH copies non-test sources] → Mitigation: the hook is on `simpleredis.go`; nil path is the production path the interpreter runs.

## Migration Plan

Test and spec delta only. Rollback is revert. No production deploy contract.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
