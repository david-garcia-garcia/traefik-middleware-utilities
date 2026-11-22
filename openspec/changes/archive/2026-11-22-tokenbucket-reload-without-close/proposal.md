# Proposal

## Why

Traefik #13618 / PR #13619 is a go-redis leak: each reload builds a `redis.NewUniversalClient` and never closes it, so goroutines and sockets accumulate. This module does not use go-redis. A live reference repro of that reload pattern (`simpleredis.New` + `NewRedis` + one `Allow`, drop without `Close`) already passed on Redis and Dragonfly; dest has no test that locks that result. Upstream: https://github.com/traefik/traefik/issues/13618 and https://github.com/traefik/traefik/pull/13619.

## What Changes

- No product change. No `Close` on the Redis limiter, no reaper or finalizer on SimpleRedis, no `reclaim/` sharing of the client.
- One live regression test in `tokenbucket/traefik_issue13618_e2e_test.go`: 50 reloads, each `simpleredis.New` + `NewRedis` through a counting TCP proxy (open sockets only; no write hold), one `Allow`, drop without `Close`. After GC, `proxy.open == 0` and leaked goroutines (excluding the proxy's own) fail only when `leakedGoroutines > 2`.
- Helpers for the proxy and the Redis key live in that file, names suffixed `13618`. Call dest `runForEachLiveEngine` / `waitLiveClient`. Transcribe dest `Allow` as `(Status, error)`. Do not copy `traefik_issues_e2e_test.go`. Do not invent an inverted control case.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: additive live proof that unreachable clients dropped without `Close` leave no proxied TCP sockets after GC and do not leave client goroutines (`leakedGoroutines > 2` fails). The MAY that a still-reachable quiet client keeps idle sockets until `Close` is unchanged.

## Impact

- New test file only: `tokenbucket/traefik_issue13618_e2e_test.go`.
- Reuses `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`).
- CI `e2e-redis` / `e2e-dragonfly` run the test; `Unit` / `Unit race` skip it via `-short`.
- Catalog after archive: `std_go_simpleredis_tcp-session` gains one requirement. `std_go_tokenbucket_allow` and `std_go_ci_test-suites` stay as they are.
- Out of scope: Traefik / go-redis client; `Close` on the limiter; product change in `simpleredis`; shared helper files; write-hold proxy from #14173; restoring three-value `Allow`.
