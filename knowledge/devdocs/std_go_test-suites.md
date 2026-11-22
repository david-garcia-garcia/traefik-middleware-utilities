# Test suites

## Language

**Lint**:
The CI job that runs golangci-lint v1.63.4. It does not execute tests and does not start Redis, Dragonfly, or Valkey.
_Avoid_: calling lint “test”; treating a green lint job as proof of Redis behaviour; enabling `testpackage`; using golangci-lint `latest` or v2

**Unit Go**:
Compiled `go test` against in-process fakes (fake TCP Redis, in-memory limiters). Needs no backend services. CI job `test` (`Unit`) passes `-short` so live files skip and `TestAlloc*` run. Ubuntu CI has gcc; local Windows without gcc cannot run `-race`.
_Avoid_: starting Redis in the unit job; proving engine compatibility here

**Unit race**:
The CI job `race` (`Unit race`) that runs the same unit suite under `go test -race -short`. Shared pool state is checked. `TestAlloc*` skip because the detector inflates B/op. A green race job is not proof of in-use-turn or idle-cap accounting.
_Avoid_: putting `-race` on the plain `test` job; putting `-race` on live Go E2E jobs; treating skip of `TestAlloc*` as those ceilings being gone

**Go E2E**:
Compiled and Yaegi tests that talk to live Redis 7, Dragonfly, and/or Valkey. CI splits them into one job per engine. Each live test reads `<PACKAGE>_LIVE_<ENGINE>[<suffix>]` (`SIMPLEREDIS_LIVE_VALKEY_AUTH`) for every engine in `liveEngineNames` and runs one subtest per address that is set, in the order redis, dragonfly, valkey. No address set skips.
_Avoid_: calling this Pester; proving malformed RESP here; treating one green engine job as another engine; adding an engine by hand-editing call sites instead of `liveEngineNames`

**Go E2E Redis**:
The CI job `e2e-redis` (`Go E2E Redis`) that starts Redis 7 `:6379`, passworded Redis `:6381`, and TLS Redis `:6383` (container `simpleredis-redis-tls`, client certificate required, ACL user `app`) and sets Redis LIVE env only.
_Avoid_: starting another engine in this job; treating a green Redis check as Dragonfly or Valkey proof

**Go E2E Dragonfly**:
The CI job `e2e-dragonfly` (`Go E2E Dragonfly`) that starts Dragonfly `:6380`, passworded Dragonfly `:6382`, and TLS Dragonfly `:6384` (container `simpleredis-dragonfly-tls`, client certificate required, ACL user `app`) and sets Dragonfly LIVE env only.
_Avoid_: starting another engine in this job; treating a green Dragonfly check as Redis or Valkey proof

**Go E2E Valkey**:
The CI job `e2e-valkey` (`Go E2E Valkey`) that starts Valkey 9 `:6385`, passworded Valkey `:6386`, and TLS Valkey `:6387` (container `simpleredis-valkey-tls`, client certificate required, ACL user `app`) and sets Valkey LIVE env only.
_Avoid_: starting another engine in this job; treating a green Redis check as Valkey proof because Valkey forked Redis

**Pester**:
`Test-Integration.ps1`: Traefik v3.7.11 plus local plugins over Docker Compose. HTTP/plugin proof. Not a substitute for compiled Go E2E. Reclaim is job `integration` (`Integration Tests`). SimpleRedis and the rate limiters are one file each; the engine is `-Engine` / `INTEGRATION_ENGINE`, matching Go E2E's live jobs.
_Avoid_: using Pester as the SimpleRedis verb or limiter-script proof; duplicating an It per engine; treating one green engine Pester job as another engine

**Pester rate limiters**:
`scripts/integration-tests.ratelimit.Tests.ps1` against the `e2e/ratelimitprobe` plugin. It proves that `tokenbucket` and `windowcounter` load under Traefik's own Yaegi, that two middleware instances on one key prefix (`/<engine>-<limiter>/a` and `/b`) share one budget through the engine, that keys stay apart, that the state lands in the engine under the prefix, and that a deny is 429 with `Retry-After` and `X-RateLimit-*`.
_Avoid_: asserting refill timing, sliding weights, or buffered sync here (Go E2E owns limiter math); running it after the SimpleRedis file on the same engine (see Gotchas)

**Pester Redis**:
The CI job `integration-redis` (`Integration Tests Redis`) that runs `./Test-Integration.ps1 -Suite simpleredis,ratelimit -Engine redis`.
_Avoid_: starting this job as Dragonfly or Valkey proof

**Pester Dragonfly**:
The CI job `integration-dragonfly` (`Integration Tests Dragonfly`) that runs the same files with `-Engine dragonfly`.
_Avoid_: starting this job as Redis or Valkey proof

**Pester Valkey**:
The CI job `integration-valkey` (`Integration Tests Valkey`) that runs the same files with `-Engine valkey`.
_Avoid_: starting this job as Redis proof

## Overview

Four suites, ten jobs (`test` and `race` are both unit; Go E2E is Redis, Dragonfly, and Valkey; Pester is reclaim plus one job per engine running SimpleRedis and the rate limiters). Put a new proof in the suite that matches what it needs running.

## How to use

- Lint: `.github/workflows/ci.yml` job `lint` (`.golangci.yml`). CI pins golangci-lint **v1.63.4** (`version: v1.63.4`, not `latest`; not v2). `gofmt` is on; `goimports`/`gofumpt` are omitted because they need a `diff` binary Windows contributors lack. `testpackage` stays off: tests are in-package white-box (`sr.borrowSocket`, `sr.freeTurns`). `gocritic` `unnamedResult.checkExported: false` is the stronger setting (`true` checks exported functions only).
- Unit: `go test -short ./...`. CI job `test` (`Unit`): `go test -short -timeout 2m -count=1 -v ./...`. Files `zzz_{domain}_test.go` (and `zzz_{domain}_yaegi_test.go` against a compiled fake). SimpleRedis chaos pool and lifecycle stress skip under `-short`; they run on a default `go test ./simpleredis/` and on Go E2E (no `-short`).
- Unit race: CI job `race`: `go test -race -short -timeout 10m -count=1 -v ./...`. Same files as unit. `TestAlloc*` skip. Concurrent MSetEX fallback stays on (4×8). Skip only `TestYaegiErrorpath_PoolWait` (race stacks are in `github.com/traefik/yaegi/interp`, `_select` / `itype.refType`, not this client) and the SimpleRedis `TestReproSelect_*_Yaegi` probes (same reason; their `_Compiled` controls stay on), plus reclaim `TestYaegi_OpenHooksRunSleepWakeClose` and `TestYaegi_WatchPublishedDeliveredToFuncAny` (Yaegi's own `select` races on context cancel). Other interpreted error paths stay on.
- Go E2E: `go test ./...` with the LIVE addrs for the engines to hit. Files `{domain}_e2e_test.go` and `{domain}_yaegi_e2e_test.go` sit next to that domain’s `.go` and `zzz_{domain}_test.go`. Shared skip/wait helpers that are not one domain live in `{package}_e2e_test.go` (same job as `zzz_fake_redis_test.go` for the fake peer). Passworded AUTH proof uses `SIMPLEREDIS_LIVE_<ENGINE>_AUTH` (`runForEachLiveEngine(t, "_AUTH", …)`) with the same any-subset rule. TLS plus ACL-user proof uses `SIMPLEREDIS_LIVE_<ENGINE>_TLS` (same rule) and reads `ca.crt`, `client.crt`, `client.key` from `SIMPLEREDIS_LIVE_TLS_DIR`; ACL user `app` has password `apppass` (`simpleredis/zzz_tls_e2e_test.go`). The certificates are not committed: each job writes them with `SIMPLEREDIS_TLS_FIXTURE_OUT=$RUNNER_TEMP/simpleredis-tls go test -run '^TestWriteTLSFixtureFiles$' ./simpleredis/`, `docker cp`s that directory to `/tls` in the TLS container before `docker start`, waits with `docker exec … redis-cli --tls … ping` (`valkey-cli` on Valkey), then runs `ACL SETUSER app on >apppass ~* +@all`. Locally, run the same lines from `.github/workflows/ci.yml` (`docker cp` also avoids bind mounts from paths a Docker host cannot share; in Git Bash set `MSYS_NO_PATHCONV=1` or `/tls` becomes a Windows path). Each `e2e-<engine>` job sets that engine's addrs only. The live tests use fixed keys, so a rerun against an engine that kept the last run's data can fail; CI starts clean engines.
- Pester: `./Test-Integration.ps1`. Docker required. Domain files are `scripts/integration-tests.<domain>.Tests.ps1` (reclaim, simpleredis, ratelimit). Helpers are `scripts/integration-tests.utils/*.ps1` (not `*Tests.ps1`). `-Suite` takes one or more of `reclaim`, `simpleredis`, `ratelimit`, `all`. `-Suite reclaim` is reclaim only. `-Suite simpleredis,ratelimit -Engine redis|dragonfly|valkey` sets `INTEGRATION_ENGINE` and runs the rate-limit file then the SimpleRedis file once. Default (`-Suite all`) runs reclaim then both files on each of the three engines. CI jobs `integration` and `integration-<engine>` match those slices.
- Benchmarks: `zzz_bench_test.go` (compiled, fake), `zzz_bench_yaegi_test.go` (interpreted, fake), and `zzz_bench_e2e_test.go` (live, compiled) in `tokenbucket/` and `windowcounter/`, next to `simpleredis/zzz_bench_test.go` and `backendbackoff/zzz_bench_test.go`. CI does not pass `-bench`, so benchmarks only compile there. Run `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/`; live benchmarks run with `-bench Live` and the `*_LIVE_*` addrs set, and skip under `-short` like the live tests.
- Do not dump every live case into one `live_test.go`. Name the file for the domain it proves (`zzz_commands_e2e_test.go` beside `commands.go` / `zzz_commands_test.go`).
- Traefik upstream-issue regressions: `traefik_issue<id>_test.go` (unit) and `traefik_issue<id>_e2e_test.go` (live) in that domain package. Keep helpers in those files with suffix `<id>`. Do not dump them into `zzz_limiter_e2e_test.go` or a shared helper file.
- Skip under `-short` or when no engine addr is set. Each addr that is set runs that engine.
- New engine: add it to `liveEngineNames` in `simpleredis/zzz_simpleredis_e2e_test.go`, `tokenbucket/zzz_limiter_e2e_test.go`, and `windowcounter/zzz_limiter_e2e_test.go`, add an `e2e-<engine>` and an `integration-<engine>` job, the compose services (`<engine>`, `<engine>-drop`, `<engine>-auth`, the `whoami-<engine>*` and `whoami-ratelimit-<engine>` routes), and the engine to the `ValidateSet`s in `Test-Integration.ps1` and the Pester files.

## Pattern snippet

```text
limiter.go
zzz_limiter_test.go                 # unit (fake)
zzz_limiter_e2e_test.go             # live Redis + Dragonfly + Valkey
zzz_limiter_yaegi_test.go           # interpreted, fake
zzz_limiter_yaegi_e2e_test.go      # interpreted, live
zzz_traefik_issue13704_test.go      # unit regression for one upstream issue
zzz_traefik_issue13704_e2e_test.go  # live remaining TTL / engine proof
```

## Key files

- `.github/workflows/ci.yml` — jobs `lint` (golangci-lint **v1.63.4**), `test` (`Unit`, `-short`, no `-race`, no services), `race` (`Unit race`, `-race -short`), `e2e-redis` (`Go E2E Redis`, no `-race`), `e2e-dragonfly` (`Go E2E Dragonfly`, no `-race`), `e2e-valkey` (`Go E2E Valkey`, no `-race`), `integration` (Pester reclaim), `integration-redis`, `integration-dragonfly`, `integration-valkey` (Pester SimpleRedis and rate limiters on that engine)
- `simpleredis/zzz_commands_e2e_test.go`, `zzz_commands_eval_e2e_test.go`, `zzz_commands_msetex_e2e_test.go`, `zzz_pool_e2e_test.go`, `zzz_simpleredis_e2e_test.go`, `zzz_tls_e2e_test.go`, `zzz_yaegi_e2e_test.go`
- `simpleredis/zzz_tls_fixture_test.go` — the one test certificate generator (`TestWriteTLSFixtureFiles` writes it for the live engines); `simpleredis/zzz_tls_fake_test.go` — the in-process TLS fake
- `windowcounter/zzz_limiter_e2e_test.go`, `zzz_limiter_yaegi_e2e_test.go`
- `tokenbucket/zzz_limiter_e2e_test.go`, `zzz_limiter_yaegi_e2e_test.go`
- `tokenbucket/zzz_traefik_issue13704_test.go`, `zzz_traefik_issue13704_e2e_test.go` — Memory idle-reclaim and live remaining TTL
- `Test-Integration.ps1`, `scripts/integration-tests.reclaim.Tests.ps1`, `scripts/integration-tests.simpleredis.Tests.ps1`, `scripts/integration-tests.ratelimit.Tests.ps1`, `scripts/integration-tests.utils/`
- `e2e/ratelimitprobe/` — Traefik local plugin that runs `tokenbucket` or `windowcounter` per request (key `<keyPrefix>:<X-Probe-Key>`), forwards on allow, and answers 429 with `Retry-After` on deny

## Gotchas

- Unit CI does not start Redis. A test that needs a live engine belongs in `*_e2e_test.go`, not `zzz_{domain}_test.go`.
- Unit CI on Ubuntu runs `test` without `-race` (`TestAlloc*` run) and `race` with `-race` (`TestAlloc*` skip). A green race job does not prove in-use-turn or idle-cap accounting. SimpleRedis chaos/lifecycle are not in the `-short` jobs; prove them with a default `go test ./simpleredis/`.
- Pester `scripts/integration-tests.simpleredis.Tests.ps1` drives `/<engine>/<verb>` (status + body) for `INTEGRATION_ENGINE`. Exact `/redis`, `/dragonfly`, and `/valkey` are health Set+Get.
- The SimpleRedis pool-cap It (`Assert-SimpleRedisLiveCap`) leaves its TIME busy-wait scripts running after it returns: inside a script `TIME` does not advance until `busy-reply-threshold` (5s), so each 500ms hold takes about 5s on Redis 7 and Valkey 8/9 alike. The engine answers `BUSY` to whatever runs next. That is why `Test-Integration.ps1` runs the rate-limit file before the SimpleRedis file on each engine. Reclaim is `scripts/integration-tests.reclaim.Tests.ps1` and CI `integration`. Compiled `*_e2e_test.go` is the client/limiter proof.
- Auth/SELECT handshake failure is Go E2E: SELECT 99 on dest engines (`zzz_pool_e2e_test.go`); WRONGPASS on `SIMPLEREDIS_LIVE_*_AUTH`; ACL-user WRONGPASS and `redis:tls` (missing client certificate, untrusted CA) on `SIMPLEREDIS_LIVE_*_TLS` (`zzz_tls_e2e_test.go`). Malformed RESP and LOADING retry stay on fake TCP.
- `testpackage` is off on purpose. Tests live in `package reclaim` / `package simpleredis` / … so they can reach unexported pool state. Do not move them to `foo_test`.
- `unnamedResult.checkExported: false` is stronger than `true`. `true` means exported functions only. Do not “tighten” it.
- Identity `==` on SimpleRedis retry/decode sentinels is required (`errUnreachable` so pool wait is not retried). `//nolint:errorlint` at those sites; do not rewrite to `errors.Is`.
