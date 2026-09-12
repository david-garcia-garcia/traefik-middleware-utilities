# Requirement
IssueKey: 2026-09-11-simpleredis-test-02-idle-cap

## Problem
The idle-cap and release-after-`Close` branches of `release` never run in tests. The tests that look like they cover those contracts cannot fail if the guards regress: eight goroutines cannot exceed eight dials, and `Get` after `Close` returns `redis:unreachable` before `borrow`/`release`. Live Pester `/redis` and `/dragonfly` each fire one request, so they also cannot prove overlapped commands leave at most `maxIdleConns` idle sockets on either backend.

## Current (code)
- `simpleredis/simpleredis.go:26` — `maxIdleConns = 8`.
- `simpleredis/simpleredis.go:64-78` — `Close` sets `closed`, drains `idle`, closes those sockets; comment: in-flight commands finish and their sockets are closed on `release`.
- `simpleredis/simpleredis.go:210-212` — `borrow` first `closed` check returns `redis:unreachable` without dialing.
- `simpleredis/simpleredis.go:233-238` — second `closed` check after idle scan, before `dial`.
- `simpleredis/simpleredis.go:244-260` — `release` closes when not reusable, or when `sr.closed || len(sr.idle) >= maxIdleConns`; otherwise appends to `idle`.
- `simpleredis/simpleredis_test.go:313-336` — `TestConcurrentCommandsStayWithinPool` starts 8 goroutines, asserts `fake.connections() > 8` fails; does not assert `len(redis.idle) <= maxIdleConns`.
- `simpleredis/simpleredis_test.go:495-522` — `TestCloseDrainsIdleAndDoesNotRepool` `Close` then `Get`; last idle assertion runs after a `Get` that never borrowed.
- `simpleredis/simpleredis_test.go:15-50` — `startFakeRedis` accepts immediately; no delay/block helper. `startSlowRedis` / `simpleredis/bench_test.go` — not found.
- `e2e/simpleredisprobe/plugin.go` — one `SimpleRedis` per plugin; sequential verbs per request; no idle-count or socket-closed header.
- `docker-compose.yml:47-75` — `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`; whoami routes `/redis` and `/dragonfly`.
- `scripts/integration-tests.Tests.ps1:90-114` — one `Invoke-WebRequest` per path; no overlap, no idle-socket assertion.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — idle pool at most eight; scenario "Concurrent commands stay within the pool" is eight concurrent commands → at most eight TCP accepts; Close scenario is idle drain then later Get, not in-flight-then-release.
- `knowledge/devdocs/std_go_simpleredis.md` — idle cap eight after release; concurrent in-flight dials are not capped.

## Desired
- Unit-test the idle cap: more overlapping commands than `maxIdleConns`, then `len(idle) <= maxIdleConns` and excess sockets closed (not leaked).
- Unit-test release-after-`Close`: command in flight before `Close`, then that socket closed and `idle` empty.
- Cover `borrow` `:233-238` (test hook or targeted unit test), or document that race as untested.
- Repair `TestConcurrentCommandsStayWithinPool` so eight goroutines could actually fail the assertion; drop or repair the vacuous final assertion in `TestCloseDrainsIdleAndDoesNotRepool`.
- Tests MUST run against both Redis and Dragonfly (both are supported backends). Cap and Close contracts must be unit-tested as above; live overlap against both engines must not leak idle sockets beyond the cap. Extend compose + Pester `/redis` `/dragonfly`. Any EVAL in that live path MUST be Lua 5.1-safe and MUST declare KEYS (Dragonfly undeclared-key reject).

## Affected
- `simpleredis/simpleredis_test.go` (new/fixed tests; overlap helper if `bench_test.go` stays absent)
- `simpleredis/simpleredis.go` only if a test-only hook is required for the `borrow` race (no production pool-behavior change)
- `e2e/simpleredisprobe/` if live overlap needs an observable (header, path, or similar)
- `docker-compose.yml` if overlap needs extra wiring
- `scripts/integration-tests.Tests.ps1` (`/redis`, `/dragonfly`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` if scenarios must match the non-tautological idle-cap and in-flight-Close contracts

## Out of scope
- perf-01 total connection cap / wait queue (idle cap only).
- Other `simpleredisfixes/` findings (test-01, test-03–07, perf-*).
- Changing `maxIdleConns`, `idleTimeout`, `ioTimeout`, or `dialTimeout` production values.
- EVALSHA, pipelining, go-redis, miniredis, TLS, Unix sockets.

## Unknowns
- How to drive genuine overlap on DestBranch without `startSlowRedis` / `bench_test.go` (not found).
- How Pester observes idle socket count vs cap on live Redis and Dragonfly (`CLIENT LIST`, probe header, or other).
- Whether `Close` between idle scan and `dial` can be made deterministic without a test hook.

## Tensions
- Spec scenario "Concurrent commands stay within the pool" (`std_go_simpleredis_tcp-session`) encodes the same eight-goroutine/eight-accept check the finding calls tautological; usage doc says in-flight dials are not capped. Follow the finding: prove idle `<= maxIdleConns`, do not treat the current scenario as idle-cap proof. Do not implement perf-01's total cap here.
- Finding cites `startSlowRedis` in `bench_test.go`; that file is not on `origin/master`.
- Spec Close scenario does not require an in-flight command; `Close`'s comment and the finding do. Desired includes the in-flight case.
- Finding how-to-fix is compiled tests vs a fake; caller also requires live overlap on Redis and Dragonfly — both are in scope.
