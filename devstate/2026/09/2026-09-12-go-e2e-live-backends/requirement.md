# Requirement
IssueKey: 2026-09-12-go-e2e-live-backends

## Problem
CI names three jobs (lint, test, integration). Live Go tests that need Redis or Dragonfly already ride inside the `test` job’s `go test ./...`. The ticket wants a fourth suite: compiled Go tests against live Redis and Dragonfly, covering as much client and limiter behaviour as possible, documented in `knowledge/devdocs`, and always run on both engines.

## Current (code)
- `.github/workflows/ci.yml` — three jobs: `lint` (golangci-lint-action, `.golangci.yml`), `test` (`go test -timeout 2m -count=1 -v ./...` with service containers `redis:7-alpine:6379` and `dragonfly:v1.40.2` published as `:6380`, env `WINDOWCOUNTER_LIVE_*`, `TOKENBUCKET_LIVE_*`, `SIMPLEREDIS_LIVE_*` set so live files do not skip), `integration` (`./Test-Integration.ps1 -SkipDockerCleanup`).
- `README.md` Tests — documents per-package `go test ./…` plus `./Test-Integration.ps1`; notes live files skip without env or under `-short`; CI “starts both engines and sets those variables so the suite does not skip.” Does not name a separate Go e2e job.
- `simpleredis/live_test.go` — `TestLive_RedisAndDragonfly` and `TestLive_PeerCloseEOFRedial` table-drive `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`; skip `-short` or both unset. Live body: pool waiter `redis:unreachable`, `MSetEX` TTL + Get, past `MSetEXAt` miss, `CLIENT KILL` then Get. Other verbs (Get/Set/Del/MGet/Incr/Expire/Eval/AUTH) are fake-TCP in `*_test.go`, not this file.
- `simpleredis/yaegi_test.go` — Yaegi GOPATH probes against compiled fake TCP only. No `TestYaegiLive_*`.
- `windowcounter/live_test.go` — `TestLive_RedisAndDragonfly` on `WINDOWCOUNTER_LIVE_*`; skip `-short` / unset. Scenarios: exact N-then-deny, buffered two-client share, sliding boundary, Peek-then-Take.
- `windowcounter/yaegi_test.go` — `TestYaegiLive_RedisAndDragonfly` repeats those live scenarios interpreted.
- `tokenbucket/live_test.go` — `TestLive_RedisAndDragonfly` on `TOKENBUCKET_LIVE_*`; skip `-short` / unset. Scenarios: burst after idle, two-instance share, memory/Redis agree.
- `tokenbucket/yaegi_test.go` — `TestYaegiLive_RedisAndDragonfly` repeats those live scenarios interpreted.
- `reclaim/` — in-process table tests (`table_test.go`, `yaegi_test.go`). No live Redis env. Reclaim does not speak Redis.
- `Test-Integration.ps1` + `scripts/integration-tests.Tests.ps1` + `docker-compose.yml` — Pester Traefik v3.7.11 + local plugins (`e2e/reclaimprobe`, `e2e/simpleredisprobe`) against compose Redis and Dragonfly. HTTP/plugin proof, not the compiled `live_test.go` suite.
- `knowledge/devdocs/` — per-library prove-with lines (`std_go_simpleredis.md`, `std_go_windowcounter.md`, `std_go_tokenbucket.md`). `index.md` / `index_std_go.md` have no packet that catalogs CI jobs (lint / unit `go test` / Pester / live Go) or what each represents.

## Desired
1. Keep lint as golangci-lint.
2. Treat `test` as regular Go tests that need no backend services.
3. Keep `integration` as Pester (`Test-Integration.ps1`).
4. Add a Go integration / e2e suite: Go tests that need live Redis or Dragonfly. Maximize coverage of behaviour that currently exists only on fake TCP. Every test in that suite MUST run against both Redis and Dragonfly.
5. Document the coverage suites and what each represents in `knowledge/devdocs`.

## Affected
- `.github/workflows/ci.yml` (`test` job services/env vs a new live/e2e job; whether `go test` stays one invocation).
- `simpleredis/live_test.go` (and possibly new live files) — expand beyond pool/MSetEX/peer-close.
- `windowcounter/live_test.go`, `tokenbucket/live_test.go` — keep dual-engine table; expand if fake-only scenarios belong on live engines.
- Yaegi live: `windowcounter/yaegi_test.go`, `tokenbucket/yaegi_test.go`; `simpleredis/yaegi_test.go` has no live counterpart.
- `README.md` Tests.
- `knowledge/devdocs/` — new or extended packet that names the suites; `index.md` / `index_std_go.md` if a new leaf is added.
- Existing specs that pin live tests to the `test` job env (`openspec/specs/std_go_*` live-env SHALL lines) if the job split moves where CI sets those variables.

## Out of scope
- Changing limiter or SimpleRedis runtime behaviour except as needed to test it live.
- Adding Valkey, Redis 8, or extra engine images.
- Replacing Pester with Go, or proving Traefik plugin HTTP from compiled `live_test.go`.
- Live Redis tests for `reclaim/` (no Redis client).
- Inventing new product APIs solely to have something to e2e.

## Unknowns
- How far “cover as much as possible” goes: every fake-TCP command/pool/retry case, or the happy-path verbs plus current live scenarios.
- Whether CI splits into a fourth job or keeps one `go test` with services (ticket names a new suite; dest already runs live files in `test`).
- Whether Yaegi live (`TestYaegiLive_*`) is part of the new e2e suite or stays with unit `go test`.
- Build tags vs today’s env-skip + `-short` (dest pattern).
- Packet name/fold for the suite catalog (`std_go_*` vs a testing leaf).

## Tensions
- Ticket: `test` depends on nothing. Dest: `test` job starts Redis and Dragonfly and runs live files in the same `go test ./...`.
- Ticket: new Go e2e suite. Dest already has `*_live_test.go` table-driven on both engines, skipped only when env unset or `-short`.
- Ticket: maximize live coverage. Dest live SimpleRedis is a thin slice; most command, pool, retry, and malformed-RESP cases are fake TCP only.
- Ticket: document suites in `knowledge/devdocs`. Packets today tell how to prove each library, not what the four CI jobs are for.
- SimpleRedis Yaegi is fake-only; windowcounter and tokenbucket already have Yaegi live on both engines.
