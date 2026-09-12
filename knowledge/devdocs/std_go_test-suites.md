# Test suites

## Language

**Lint**:
The CI job that runs golangci-lint. It does not execute tests and does not start Redis or Dragonfly.
_Avoid_: calling lint “test”; treating a green lint job as proof of Redis behaviour

**Unit Go**:
Compiled `go test` against in-process fakes (fake TCP Redis, in-memory limiters). Needs no backend services. CI passes `-short` so live files skip.
_Avoid_: starting Redis in the unit job; proving engine compatibility here

**Go E2E**:
Compiled and Yaegi tests that talk to live Redis 7 and Dragonfly. Every case table-drives both engines. CI job `e2e` (`Go E2E`) starts the engines and sets `*_LIVE_*`, plus passworded siblings for WRONGPASS. Local `go test` without those env vars skips; exactly one addr in a pair fails.
_Avoid_: calling this Pester; proving malformed RESP here; one-engine-only runs

**Pester**:
`Test-Integration.ps1`: Traefik v3.7.11 plus local plugins over Docker Compose. HTTP/plugin proof. Not a substitute for compiled Go E2E.
_Avoid_: using Pester as the SimpleRedis verb or limiter-script proof

## Overview

Four suites, four jobs. Put a new proof in the suite that matches what it needs running.

## How to use

- Lint: `.github/workflows/ci.yml` job `lint` (`.golangci.yml`).
- Unit: `go test -short ./...`. Files `{domain}_test.go` (and `{domain}_yaegi_test.go` against a compiled fake).
- Go E2E: `go test ./...` with both `*_LIVE_REDIS` and `*_LIVE_DRAGONFLY` set. Files `{domain}_e2e_test.go` and `{domain}_yaegi_e2e_test.go` sit next to that domain’s `.go` and `{domain}_test.go`. Shared skip/wait helpers that are not one domain live in `{package}_e2e_test.go` (same job as `fake_redis_test.go` for the fake peer). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` with the same both-or-neither rule.
- Pester: `./Test-Integration.ps1`. Docker required.
- Do not dump every live case into one `live_test.go`. Name the file for the domain it proves (`commands_e2e_test.go` beside `commands.go` / `commands_test.go`).
- Skip under `-short` or both live addrs unset. Fail if exactly one addr is set.

## Pattern snippet

```text
limiter.go
limiter_test.go           # unit (fake)
limiter_e2e_test.go        # live Redis + Dragonfly
limiter_yaegi_test.go      # interpreted, fake
limiter_yaegi_e2e_test.go # interpreted, live
```

## Key files

- `.github/workflows/ci.yml` — jobs `lint`, `test` (`-short`, no services), `e2e` (`Go E2E`), `integration` (Pester)
- `simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `pool_e2e_test.go`, `simpleredis_e2e_test.go`, `yaegi_e2e_test.go`
- `windowcounter/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`
- `tokenbucket/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`
- `Test-Integration.ps1`

## Gotchas

- Unit CI does not start Redis. A test that needs a live engine belongs in `*_e2e_test.go`, not `{domain}_test.go`.
- Pester `/redis` and `/dragonfly` headers are Traefik proof. Compiled `*_e2e_test.go` is the client/limiter proof.
- Auth/SELECT handshake failure is Go E2E: SELECT 99 on dest engines (`pool_e2e_test.go`); WRONGPASS on `SIMPLEREDIS_LIVE_*_AUTH`. Malformed RESP and LOADING retry stay on fake TCP.
- Reclaim has no Redis client; it has no Go E2E files.
