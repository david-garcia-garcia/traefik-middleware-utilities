## Why

Dest CI’s `test` job starts Redis and Dragonfly and runs compiled live files in the same `go test ./...` as fake-TCP units. SimpleRedis live coverage is a thin slice; there is no usage packet that names lint, unit Go, Go E2E, and Pester.

## What Changes

- Keep `lint` as golangci-lint.
- Make `test` unit-only: no service containers, `go test -short` so live files skip.
- Keep `integration` as Pester (`Test-Integration.ps1`).
- Add GitHub job `e2e` (`Go E2E`): dest’s Redis 7 `:6379` and Dragonfly `:6380`, all `*_LIVE_*` env, `go test` without `-short`.
- Every compiled and Yaegi live case table-drives both engines. Skip only when `-short` or both addrs unset; exactly one addr set fails.
- Expand live SimpleRedis to engine-success verbs (Get/Set/Del/MGet/Incr/Expire/Eval/MSetEX, pool wait, peer-close) plus Yaegi live. Keep DestBranch SELECT 99 and WRONGPASS live proof. Expand window-counter and token-bucket live scenarios. Do not port fake-peer abuse or reclaim.
- Document the four suites in `knowledge/devdocs/std_go_test-suites.md`. Point per-library prove-with lines and README Tests at that catalog.

## Capabilities

### New Capabilities

- `std_go_ci_test-suites`: Four CI suites (lint, unit Go, Go E2E, Pester), job split, skip/fail rules, usage packet.
- `std_go_simpleredis_live-e2e`: Compiled and Yaegi live SimpleRedis on Redis and Dragonfly covering engine-success verbs; fake-TCP stays the peer-abuse proof.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: Live MSetEX CI is the `e2e` job, not `test`.
- `std_go_simpleredis_tcp-session`: Compiled pool-wait and CLIENT KILL live files run in `e2e` (same skip/fail as the suite).
- `std_go_windowcounter_sync-flush`: Live skip/fail and extra Peek/expire scenarios; CI is `e2e`.
- `std_go_tokenbucket_lua-eval`: Live skip/fail and refund-when-wait-exceeds-maxDelay; CI is `e2e`.

## Impact

- `.github/workflows/ci.yml` (split services/env onto `e2e`; `test` uses `-short`).
- `simpleredis/{commands,commands_eval,commands_msetex,pool,simpleredis}_e2e_test.go`, `yaegi_e2e_test.go`.
- `windowcounter/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`.
- `tokenbucket/limiter_e2e_test.go`, `limiter_yaegi_e2e_test.go`.
- `README.md` Tests; `knowledge/devdocs/std_go_test-suites.md` plus index rows; prove-with lines on SimpleRedis, windowcounter, tokenbucket packets.
- Existing live-env SHALL lines after archive.
- No runtime API change. No new Redis/Dragonfly images. Pester compose unchanged.
