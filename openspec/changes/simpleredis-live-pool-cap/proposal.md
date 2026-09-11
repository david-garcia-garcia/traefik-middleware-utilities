## Why

On DestBranch, SimpleRedis caps only the idle list at eight. Concurrent callers dial without a live-socket bound, then `release` closes anything that would make idle exceed eight. Bursty Traefik traffic therefore pays a TCP handshake (and AUTH/SELECT) on most commands, and Redis `maxclients` is the only backstop.

## What Changes

- Cap **live** sockets (`poolSize` default 8) separately from the idle list. Callers above the cap wait on a buffered `chan struct{}` instead of dialing.
- Bound that wait with `poolTimeout` default 1s. Elapse returns `redis:unreachable`. Do not queue forever.
- Keep idle trimming at eight, but do not close a reusable socket only because idle is full while live sockets are under `poolSize`.
- `Init(host, pass, database)` stays three arguments. No public pool knobs. No `go-redis` import.
- Prove the cap and timeout on a fake server **and** on live Redis and Dragonfly (compiled tests plus dest compose + Pester). Fake-server tests are not a substitute.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: concurrent commands wait for a live slot instead of dialing past eight; a full pool times out as `redis:unreachable`; Traefik e2e on `/redis` and `/dragonfly` MUST observe at most eight live clients and a waiter error.

## Impact

- `simpleredis/simpleredis.go` (`borrow`, `release`, live count / semaphore, constants).
- `simpleredis/simpleredis_test.go` plus a live test file gated on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`.
- `e2e/simpleredisprobe/`, `scripts/integration-tests.Tests.ps1`, CI `test` job env for those live addresses.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` (uncapped in-flight gotcha).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Reclaim routes `/a` `/b` stay. Sibling findings in `simpleredisfixes/` are other tickets.
