# Requirement
IssueKey: 2026-09-11-simpleredis-test-07-benchmarks

## Problem
SimpleRedis on `master` has no benchmark, no allocation guard, no large-value decode measurement, and no connection-churn test. CI `go test` does not run `Benchmark` functions, so encode/decode allocation regressions and buffer-growth on a 100 KB bulk would not fail the suite.

## Current (code)
- `simpleredis/` on `origin/master` (`7dc4b051`) — `simpleredis.go`, `simpleredis_test.go`, `yaegi_test.go`, `LICENSE` only. No `bench_test.go`. No `interpretedcost_test.go`.
- `simpleredis/simpleredis_test.go` `startFakeRedis` — takes `*testing.T`, not `testing.TB`; compiled tests only.
- `simpleredis/yaegi_test.go` `writeGopathSimpleredis` / `writeGopathFile` — take `*testing.T`.
- `.github/workflows/ci.yml` `test` job — `go test -timeout 2m -count=1 -v ./...` (no `-bench`). Service containers `redis:7-alpine` `:6379` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` `:6380`.
- `.github/workflows/ci.yml` `integration` job — `./Test-Integration.ps1`; failure logs dump `redis`, `dragonfly`, `whoami-redis`, `whoami-dragonfly`.
- `scripts/integration-tests.Tests.ps1` — `GET /redis` and `GET /dragonfly` assert Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval headers.
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379`; `dragonfly` `v1.40.2` at `dragonfly:6379`; PathPrefix `/redis` and `/dragonfly`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — Pester every-verb scenarios on Redis and Dragonfly; Eval Lua 5.1-safe with KEYS. No alloc/bench requirement.
- `knowledge/devdocs/std_go_simpleredis.md` — prove with `go test ./simpleredis/...` and `./Test-Integration.ps1` (Redis and Dragonfly). No allocation-guard instruction.
- Reviewer working tree (not dest) may hold `simpleredis/bench_test.go` from the review; dest does not.

## Desired
- Land the finding's measurement files (compiled encode/decode/end-to-end benches, churn tests) and a 100 KB `Set`/`Get` decode benchmark. Every current measurement uses small values.
- CI allocation guards on `allocs/op` and `B/op`, not `ns/op`. Do not wait for perf-01 to land those guards.
- Keep the two churn tests. If they cannot become a hard pool-cap assertion yet, record that on `explore.md` later and still ship the rest.
- Keep live verb coverage on both engines in CI: compose + Pester `/redis` and `/dragonfly`. Lua 5.1-safe. Dragonfly KEYS required.
- Widen `startFakeRedis` (and Yaegi GOPATH helpers if benches reuse them) to `testing.TB` so benchmarks can call them. No production `simpleredis.go` change for this finding.

## Affected
- `simpleredis/simpleredis_test.go` (`startFakeRedis` signature)
- `simpleredis/yaegi_test.go` if interpreted-cost benches reuse GOPATH helpers
- new `simpleredis/bench_test.go` (and `interpretedcost_test.go` if this ticket ships those measurements)
- `.github/workflows/ci.yml` only if guards need a `-bench` step or an extra Test entry; do not drop Redis/Dragonfly services or the integration job
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` only if propose adds alloc-guard scenarios
- `scripts/integration-tests.Tests.ps1`, `docker-compose.yml`, `e2e/simpleredisprobe/` — keep `/redis` and `/dragonfly` verb coverage; do not remove

## Out of scope
- perf-01 bounded pool / production connection-cap changes
- perf-05 EVALSHA, perf-06 single-write encode, perf-07 readSlice, perf-08 unsafe zero-copy
- CI gates on `ns/op` / wall-clock
- Changing Init/Close/pool behaviour, TLS, go-redis, miniredis
- Rate limiters, window counters, token bucket
- Dropping or skipping Pester `/redis` or `/dragonfly`

## Unknowns
- Guard mechanism in CI: `testing.AllocsPerRun` Test functions vs `go test -bench` with a golden vs benchstat (dest CI has no `-bench`).
- Whether new allocation-guard tests must themselves dial live Redis and Dragonfly, or fake-server benches plus existing Pester is enough (HARD REQUIREMENT names compose + Pester `/redis` `/dragonfly`).
- Numeric `allocs/op` and `B/op` budgets: finding numbers are Go 1.25.6 windows/amd64; CI is Go 1.21 ubuntu-latest.
- Whether `interpretedcost_test.go` (Yaegi encode/unsafe/`Get` benches) is in this ticket's ship set (finding "What was added" vs "Still to do").
- Exact 100 KB decode fixture shape (bulk GET reply vs full Set-then-Get round trip).

## Tensions
- Finding status "partially applied — measurement files were added during this review" vs dest `origin/master` which has neither file — this ticket must land them, not assume dest already has them.
- Finding: turn churn into assertions once perf-01 lands. Caller: do not wait for perf-01; if churn cannot be a hard cap assertion yet, record on explore.md and still ship CI guards, large-value decode, and the churn tests.
- Reviewer `bench_test.go` calls `startFakeRedis(b, …)` but dest `startFakeRedis` is `*testing.T` only — will not compile until widened.
- Dest CI `go test` without `-bench` never runs `Benchmark*` — alloc guards that exist only as benchmarks would not run in CI.
