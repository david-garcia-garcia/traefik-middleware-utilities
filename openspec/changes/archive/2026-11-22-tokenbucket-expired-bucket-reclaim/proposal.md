# Proposal

## Why

Traefik's in-memory `ttlmap` keeps every distinct source until the map hits 65 536 entries ([traefik/traefik#13704](https://github.com/traefik/traefik/issues/13704)). This library already reclaims expired Memory sources when a new key arrives, and Redis/Dragonfly already set integer-second `EXPIRE` on each EVAL. Those paths have no regression test for the ten-idle-plus-one-new case, and live Allow does not observe remaining TTL.

## What Changes

- No product change. `Memory.Allow`, `makeRoomLocked`, `allowScript`, and `Redis.Allow` stay as they are.
- Unit regression: `tokenbucket/traefik_issue13704_test.go` (`TestTraefik13704_MemoryReclaimsExpiredBuckets`). Ten sources idle past `ttl` plus one new source leave exactly one stored bucket.
- Live regression: `tokenbucket/traefik_issue13704_e2e_test.go` (`TestTraefik13704_RedisBucketExpires`). After Allow, the bucket key's `PTTL` is in `(0, ttl]` on Redis and on Dragonfly. Same gating as `TestLive_Limiter`.
- Spec delta: one new scenario under the existing Memory cap/expiry requirement. No `PTTL` SHALL. No new debt naming `dropExpired`/`dropOne`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: add a scenario to "Memory store caps sources and removes expired ones first" — ten sources idle past `ttl` plus one new source leave exactly one stored source. The SHALL is unchanged.

## Impact

- **Code**: none (`tokenbucket/memory.go`, `tokenbucket/lua.go`, `tokenbucket/redis.go` are the behavior under test).
- **Tests**: new `tokenbucket/traefik_issue13704_test.go` and `tokenbucket/traefik_issue13704_e2e_test.go`. Reuse `runForEachLiveEngine` / `waitLiveClient`. Helpers `liveKey13704` and `evalKey13704` live in the e2e file. Existing `tokenbucket/memory_test.go` cap tests and `TestLive_Limiter` stay.
- **Specs**: delta under `std_go_tokenbucket_allow` only. `std_go_tokenbucket_lua-eval` already requires integer-second `EXPIRE`; live tests observe remaining TTL.
- **Unchanged**: public API, CI jobs, Yaegi extra file, `backendbackoff.Gate`, `knowledge/debt/` for Memory `dropExpired`/`dropOne`.
