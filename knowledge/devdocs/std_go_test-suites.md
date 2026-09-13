# Test suites

## Language

**Lint**:
The CI job that runs golangci-lint. It does not execute tests and does not start Redis or Dragonfly.
_Avoid_: calling lint “test”; treating a green lint job as proof of Redis behaviour

**Unit Go**:
Compiled `go test` against in-process fakes (fake TCP Redis, in-memory limiters). Needs no backend services. CI job `test` (`Unit`) passes `-short` so live files skip and `TestAlloc*` run. Ubuntu CI has gcc; local Windows without gcc cannot run `-race`.
_Avoid_: starting Redis in the unit job; proving engine compatibility here

**Unit race**:
The CI job `race` (`Unit race`) that runs the same unit suite under `go test -race -short`. Shared pool state is checked. `TestAlloc*` skip because the detector inflates B/op. A green race job is not proof of in-use-turn length or idle-cap accounting.
_Avoid_: putting `-race` on the plain `test` job; putting `-race` on live Go E2E jobs; treating skip of `TestAlloc*` as those ceilings being gone

**Go E2E**:
Compiled and Yaegi tests that talk to live Redis 7 and/or Dragonfly. CI splits them into two jobs. Local `go test` without those env vars skips; one addr in a pair runs that engine only; both addrs run both.
_Avoid_: calling this Pester; proving malformed RESP here; treating one green engine job as the other engine

**Go E2E Redis**:
The CI job `e2e-redis` (`Go E2E Redis`) that starts Redis 7 `:6379` and passworded Redis `:6381` and sets Redis LIVE env only.
_Avoid_: starting Dragonfly in this job; treating a green Redis check as Dragonfly proof

**Go E2E Dragonfly**:
The CI job `e2e-dragonfly` (`Go E2E Dragonfly`) that starts Dragonfly `:6380` and passworded Dragonfly `:6382` and sets Dragonfly LIVE env only.
_Avoid_: starting Redis in this job; treating a green Dragonfly check as Redis proof

**Pester**:
`Test-Integration.ps1`: Traefik v3.7.11 plus local plugins over Docker Compose. HTTP/plugin proof. Not a substitute for compiled Go E2E. Reclaim is job `integration` (`Integration Tests`). SimpleRedis is one file; Redis vs Dragonfly is `-Engine` / `INTEGRATION_ENGINE`, matching Go E2E’s two live jobs.
_Avoid_: using Pester as the SimpleRedis verb or limiter-script proof; duplicating Redis and Dragonfly Its; treating one green engine Pester job as the other engine

**Pester Redis**:
The CI job `integration-redis` (`Integration Tests Redis`) that runs `./Test-Integration.ps1 -Suite simpleredis -Engine redis`.
_Avoid_: starting this job as Dragonfly proof

**Pester Dragonfly**:
The CI job `integration-dragonfly` (`Integration Tests Dragonfly`) that runs the same SimpleRedis file with `-Engine dragonfly`.
_Avoid_: starting this job as Redis proof

## Overview

Four suites, eight jobs (`test` and `race` are both unit; Go E2E is Redis and Dragonfly; Pester is reclaim plus Redis and Dragonfly). Put a new proof in the suite that matches what it needs running.

## How to use

- Lint: `.github/workflows/ci.yml` job `lint` (`.golangci.yml`).
- Unit: `go test -short ./...`. CI job `test` (`Unit`): `go test -short -timeout 2m -count=1 -v ./...`. Files `{domain}_test.go` (and `{domain}_yaegi_test.go` against a compiled fake).
- Unit race: CI job `race`: `go test -race -short -timeout 10m -count=1 -v ./...`. Same files as unit. `TestAlloc*` skip.
- Go E2E: `go test ./...` with the LIVE addrs for the engines to hit. Files `{domain}_e2e_test.go` and `{domain}_yaegi_e2e_test.go` sit next to that domain’s `.go` and `{domain}_test.go`. Shared skip/wait helpers that are not one domain live in `{package}_e2e_test.go` (same job as `fake_redis_test.go` for the fake peer). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` with the same one-or-both rule. CI job `e2e-redis` sets Redis addrs only; `e2e-dragonfly` sets Dragonfly addrs only.
- Pester: `./Test-Integration.ps1`. Docker required. Domain files are `scripts/integration-tests.<domain>.Tests.ps1` (reclaim, simpleredis). Helpers are `scripts/integration-tests.utils/*.ps1` (not `*Tests.ps1`). `-Suite reclaim` is reclaim only. `-Suite simpleredis -Engine redis|dragonfly` sets `INTEGRATION_ENGINE` and runs the SimpleRedis file once. Default (`-Suite all`) runs reclaim then both engines. CI jobs `integration`, `integration-redis`, and `integration-dragonfly` match those three slices.
- Do not dump every live case into one `live_test.go`. Name the file for the domain it proves (`commands_e2e_test.go` beside `commands.go` / `commands_test.go`).
- Skip under `-short` or both live addrs unset. One addr set runs that engine only.

## Pattern snippet

```text
limiter.go
limiter_test.go           # unit (fake)
limiter_e2e_test.go        # live Redis + Dragonfly
limiter_yaegi_test.go      # interpreted, fake
limiter_yaegi_e2e_test.go # interpreted, live
```

## Key files

- `.github/workflows/ci.yml` — jobs `lint`, `test` (`Unit`, `-short`, no `-race`, no services), `race` (`Unit race`, `-race -short`), `e2e-redis` (`Go E2E Redis`, no `-race`), `e2e-dragonfly` (`Go E2E Dragonfly`, no `-race`), `integration` (Pester reclaim), `integration-redis` (Pester SimpleRedis Redis), `integration-dragonfly` (Pester SimpleRedis Dragonfly)
- `simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `pool_e2e_test.go`, `simpleredis_e2e_test.go`, `yaegi_e2e_test.go`
- `windowcounter/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`
- `tokenbucket/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`
- `Test-Integration.ps1`, `scripts/integration-tests.reclaim.Tests.ps1`, `scripts/integration-tests.simpleredis.Tests.ps1`, `scripts/integration-tests.utils/`

## Gotchas

- Unit CI does not start Redis. A test that needs a live engine belongs in `*_e2e_test.go`, not `{domain}_test.go`.
- Unit CI on Ubuntu runs `test` without `-race` (`TestAlloc*` run) and `race` with `-race` (`TestAlloc*` skip). A green race job does not prove `inUseTurns` length or idle-cap accounting.
- Pester `scripts/integration-tests.simpleredis.Tests.ps1` drives `/<engine>/<verb>` (status + body) for `INTEGRATION_ENGINE`. Exact `/redis` and `/dragonfly` are health Set+Get. Reclaim is `scripts/integration-tests.reclaim.Tests.ps1` and CI `integration`. Compiled `*_e2e_test.go` is the client/limiter proof.
- Auth/SELECT handshake failure is Go E2E: SELECT 99 on dest engines (`pool_e2e_test.go`); WRONGPASS on `SIMPLEREDIS_LIVE_*_AUTH`. Malformed RESP and LOADING retry stay on fake TCP.
- Reclaim has no Redis client; it has no Go E2E files. Backend backoff has no store; it has no Go E2E files and no `*_LIVE_*` variable.
