## Why

Dest retries a dead pooled SimpleRedis socket once unless the error is a timeout, but the only stale-conn test closes the client fd (`os.ErrClosed` on `SetDeadline`) and never maps `io.EOF`. Compose/Pester `/redis` and `/dragonfly` are happy-path verbs, so a silent regression of peer-close recovery would ship.

## What Changes

- Add compiled fake-TCP tests that close the **accepted** socket after the first reply (do not close the client). Success: first Get ok, second Get ok on a new dial, two accepts, dead conn not in `idle`. Fail: listener closed so retry `borrow` returns `redis:unreachable`.
- Rename `TestStaleConnectionIsRetried` so it names the client-fd / `SetDeadline` / `os.ErrClosed` arm.
- Prove the same recovery live on Redis and Dragonfly: Go tests (`SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`, skip `-short` / unset) plus Pester Its on `/redis` and `/dragonfly` after `CLIENT KILL ADDR|ID`.
- Probe `recover=1` runs Set+Get only and sets `X-SimpleRedis-Recover: ok`. Default verb headers stay. `Init` stays in `New`. No production `ClientKill`. No new Lua; existing EVAL stays Lua 5.1-safe with keys in `KEYS`.
- Do not change retry policy unless those tests fail on dest `exec`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: add a peer-closed idle scenario (`io.EOF` → one retry, distinct from client-side close); retry `borrow` failure is `redis:unreachable`; live Redis and Dragonfly plus Traefik `/redis` `/dragonfly` MUST prove recovery after `CLIENT KILL`.

## Impact

- `simpleredis/simpleredis_test.go` (new peer-close helper; rename existing stale test). `simpleredis.go` only if tests prove dest `exec` wrong.
- New `simpleredis` live test file (same skip/env shape as `windowcounter/live_test.go`). CI env next to the windowcounter live addrs.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`. Compose stays `timeout` 0; no container restart.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Reclaim `/a` `/b` and SimpleRedis default verb headers unchanged.
