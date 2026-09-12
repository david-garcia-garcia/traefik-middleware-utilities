# Requirement
IssueKey: 2026-09-11-simpleredis-perf-01-pool-cap

## Problem
SimpleRedis caps only the idle list (`maxIdleConns = 8`). `borrow` dials whenever that list is empty, so live sockets scale with concurrent callers. `release` then closes any socket that would make idle exceed eight. Bursty Traefik traffic therefore pays a TCP handshake (and AUTH/SELECT when those Init fields are set) on most commands, and Redis `maxclients` is the only backstop.

## Current (code)
- `simpleredis/simpleredis.go:26` — `maxIdleConns = 8`; no `poolSize` or `poolTimeout`.
- `simpleredis/simpleredis.go:54-62` — `SimpleRedis` holds `idle []*pooledConn` and `closed`; no live-socket count and no wait semaphore.
- `simpleredis/simpleredis.go:203-242` — `borrow` reuses an idle socket younger than `idleTimeout`, else `dial()` with no total-cap check (`:239-241`).
- `simpleredis/simpleredis.go:244-260` — `release` closes the socket when `closed` or `len(idle) >= maxIdleConns`, even if fewer than eight are live.
- `simpleredis/simpleredis.go:262-289` — `dial` always AUTH then SELECT when `pass` / `database` are set.
- `simpleredis/simpleredis_test.go:313-336` — `TestConcurrentCommandsStayWithinPool` runs eight goroutines and asserts `fake.connections() <= 8`; it never drives concurrency *above* eight, so an uncapped dialer still passes.
- `simpleredis/bench_test.go` — `not found` on `origin/master` (`TestConnectionChurnAcrossBursts` / `TestConnectionChurnUnderLatency` named in the finding are not dest tests).
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — idle pool of at most eight; “Concurrent commands SHALL not open more than eight connections”; scenario only covers eight concurrent commands against a fake.
- `knowledge/devdocs/std_go_simpleredis.md` — gotcha: idle cap eight after release; concurrent in-flight dials are not capped.
- `e2e/simpleredisprobe/plugin.go` — one client, `Init` in `New`, sequential verbs per request; no pool-cap or pool-timeout observation.
- `scripts/integration-tests.Tests.ps1` — Pester `/redis` and `/dragonfly` assert verb headers only.
- `Test-Integration.ps1`, `docker-compose.yml`, `.github/workflows/ci.yml` — compose Redis `redis:7-alpine` + Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`; CI `integration` job runs `Test-Integration.ps1`; CI `test` job already starts both engines for other packages.

## Desired
- Total live connection cap (`poolSize`) separate from the idle cap; default in the finding’s 8–16 range. Callers above the cap wait on a buffered `chan struct{}` semaphore (Yaegi-safe stdlib) instead of dialing.
- `poolTimeout`: if no connection frees in that window, return `redis:unreachable` or a distinct `redis:pool-timeout` (finding allows either). Do not queue without bound.
- Keep idle trimming, but do not close a reusable socket only because the idle list is full while live sockets are under `poolSize`.
- Compiled tests: concurrency above the cap never exceeds `poolSize` live sockets; pool timeout when all slots are busy; burst dials stay at or near `poolSize` (finding names `TestConnectionChurnAcrossBursts` once that file exists on dest).
- **E2E (caller addendum):** prove this ticket’s new pool behaviour on **both** Redis and Dragonfly. Fake-server tests are not a substitute. Extend dest compose + Pester (`Test-Integration.ps1`, `/redis` and `/dragonfly`, `e2e/simpleredisprobe`). Any Lua this change sends MUST be Lua 5.1-safe (no `table.maxn`) and MUST list touched keys in KEYS. CI MUST exercise both backends.

## Affected
- `simpleredis/simpleredis.go` (`borrow`, `release`, session fields / constants)
- `simpleredis/simpleredis_test.go` (and any new dest test file this change adds)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (concurrent-cap / wait / timeout)
- `knowledge/devdocs/std_go_simpleredis.md` (idle-only-cap gotcha)
- `e2e/simpleredisprobe/` (prove the new behaviour under Yaegi on each engine)
- `scripts/integration-tests.Tests.ps1`
- `Test-Integration.ps1` / `docker-compose.yml` / `.github/workflows/ci.yml` only if the e2e proof needs a new route, wait, or CI log dump; keep existing reclaim `/a` `/b` and both engines

## Out of scope
- Sibling findings in `simpleredisfixes/` (perf-02 I/O-timeout fan-out, idle-reaper, pipelining, EVALSHA, encode/decode, coverage tickets, MSETEX).
- Importing `go-redis` or miniredis; TLS; Unix sockets.
- Changing `Init(host, pass, database)` unless explore finds a const-only pool is insufficient.
- Rate limiters, window counter, token bucket.
- Changing the dest Dragonfly image pin or Lua 5.1/KEYS rules already owned by EVAL e2e.

## Unknowns
- Exact `poolSize` default (finding says 8–16) and `poolTimeout` duration.
- Whether the timeout error string is `redis:unreachable` or a new `redis:pool-timeout`.
- How Pester / the probe observes live socket count and pool-timeout on `/redis` and `/dragonfly` (headers, `CLIENT LIST`, concurrent requests).
- Whether CI `test` job live Redis/Dragonfly (`127.0.0.1:6379` / `6380`) should also run compiled pool tests in addition to Pester (caller named compose + Pester as the required proof).
- go-redis `internal/pool/pool.go` shape is cited by the finding; `knowledge/research/` has no pool-cap finding (only EVAL/script notes).

## Tensions
- Spec already says concurrent commands SHALL not open more than eight; dest code and the usage-doc gotcha do not enforce a total cap. This ticket asks for a wait-queue cap whose default may be 8–16, which may reshape the spec’s “eight”.
- Finding’s proof plan is fake-server churn tests; caller addendum says fake-server is not a substitute and requires Redis + Dragonfly via dest Pester — follow the addendum (fake tests may still exist; they cannot be the only proof).
- Index suggested order pairs perf-01 with perf-02; this run is bound to perf-01 only.
