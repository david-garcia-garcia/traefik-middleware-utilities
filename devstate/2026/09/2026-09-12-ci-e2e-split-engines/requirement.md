# Requirement
IssueKey: 2026-09-12-ci-e2e-split-engines

## Problem
GitHub Actions has one `Go E2E` check (`e2e`) that starts Redis and Dragonfly together. A Redis-only or Dragonfly-only live failure is the same red check. The ticket wants two Checks: one live-Go suite on Redis, one on Dragonfly.

## Current (code)
- `.github/workflows/ci.yml` — five jobs: `lint` (`Lint`), `test` (`Unit`, `go test -short`), `race` (`Unit race`, `go test -race -short`), `e2e` (`Go E2E`, services `redis:7-alpine:6379` and `dragonfly:v1.40.2` as `:6380`, passworded `docker run` siblings `:6381`/`:6382`, all `*_LIVE_REDIS` and `*_LIVE_DRAGONFLY` plus AUTH pairs set, `go test` without `-short`), `integration` (`Integration Tests`, Pester).
- `simpleredis/simpleredis_e2e_test.go`, `windowcounter/limiter_e2e_test.go`, `tokenbucket/limiter_e2e_test.go` — `lookupLiveEngineAddrs` skips when both addrs unset or `testing.Short`; `t.Fatal` when exactly one addr is set (`errLiveEngineOneAddr`). `TestLookupLiveEngineAddrs` proves that both-or-neither rule under `-short`.
- `simpleredis/pool_e2e_test.go` — AUTH live cases table-drive `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` through the same helper.
- `openspec/specs/std_go_ci_test-suites/spec.md` — CI SHALL run one `e2e` named `Go E2E` that starts both engines and sets every LIVE address; live tests MUST fail when exactly one address in a pair is set; each live case SHALL execute against Redis and against Dragonfly.
- Same both-engines / fail-one-addr pin: `openspec/specs/std_go_simpleredis_live-e2e/spec.md`, `std_go_simpleredis_resp-commands/spec.md`, `std_go_simpleredis_tcp-session/spec.md`, `std_go_tokenbucket_lua-eval/spec.md`, `std_go_windowcounter_sync-flush/spec.md`.
- `knowledge/devdocs/std_go_test-suites.md` — Go E2E is one job; “exactly one addr in a pair fails”; “Avoid: one-engine-only runs”.
- `README.md` Tests — one Go E2E invocation; skip both-unset; fail if exactly one addr is set.

## Desired
1. GitHub Checks show two live-Go items: one suite against Redis, one against Dragonfly, so an engine-only failure is its own check.
2. Unit (`go test -short`), Unit race, Lint, and Pester Integration Tests stay as they are unless a change is required so each live job can skip the other engine.

## Affected
- `.github/workflows/ci.yml` job `e2e` (split into two jobs/checks; services, AUTH `docker run`, env).
- `lookupLiveEngineAddrs` / `liveEngineAddrs` / `TestLookupLiveEngineAddrs` in `simpleredis/simpleredis_e2e_test.go`, `windowcounter/limiter_e2e_test.go`, `tokenbucket/limiter_e2e_test.go` (and AUTH call sites in `simpleredis/pool_e2e_test.go`) if one-engine CI must not `Fatal`.
- Specs and usage that pin one `e2e` job and both-or-neither fail: `openspec/specs/std_go_ci_test-suites/spec.md` and the live-e2e SHALL lines listed under Current; `knowledge/devdocs/std_go_test-suites.md`; `README.md` Tests.

## Out of scope
- New live scenarios or extra engines (Valkey, Redis 8).
- Changing limiter or SimpleRedis runtime behaviour.
- Replacing Pester or proving Traefik HTTP from compiled e2e.
- Changing Lint, Unit, Unit race, or Integration Tests except skip/fail helpers needed so a Redis job does not require Dragonfly env (and the reverse).
- Race on the live jobs.

## Unknowns
- Exact GitHub Check `name:` / job ids (ticket requires two visible items, Redis vs Dragonfly; does not quote the strings).
- Whether passworded AUTH containers (`:6381` / `:6382`) split with the same jobs (must, if AUTH env still both-or-neither).
- Whether a Redis job may publish Redis on `:6379` only (no Dragonfly `:6380` on that runner).

## Tensions
- Ticket: two Checks, one engine each. Spec `std_go_ci_test-suites`: one `e2e` job starts both engines, sets all six LIVE vars, live tests MUST NOT skip.
- Ticket: each live job skips the other engine. Code and `TestLookupLiveEngineAddrs`: exactly one addr is `errLiveEngineOneAddr` / `t.Fatal`. Usage packet and README: fail if exactly one addr is set; avoid one-engine-only runs.
- Ticket: Unit stays unless skip helpers must change. `TestLookupLiveEngineAddrs` (`onlyRedis` / `onlyDragonfly`) lives in e2e files but runs under `-short` in the Unit job; inverting the fail rule changes those unit assertions.
