# Design

## Context

See `proposal.md` — Why. Binding brief: no product change; regression tests only. Proceed policies: `devstate/explore.md`. Spec: `std_go_tokenbucket_allow` (fold one scenario into "Memory store caps sources and removes expired ones first"). `std_go_tokenbucket_lua-eval` already requires integer-second `EXPIRE`; do not add a `PTTL` SHALL.

Dest already reclaims: `Memory.Allow` drops a present-but-expired entry, then `makeRoomLocked` pops expired heads before storing a missing key. Redis EVAL ends with `redis.call('expire', key, ttl)` (`ttlSeconds` as ARGV[3]). `Allow` returns `(Status, error)`. Live gating lives in `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`). Cap/expiry at 65536 is already in `tokenbucket/memory_test.go`; this change does not replace those tests.

Reference bodies (read-only, other tree): `TestTraefik13704_MemoryReclaimsExpiredBuckets`, `TestTraefik13704_RedisBucketExpires`. Those call the old three-value `Allow`. Do not copy that tree or other issues' helpers.

## Goals / Non-Goals

**Goals:**

- Pin Memory: ten idle sources past `ttl` plus one new source leave exactly one stored bucket.
- Observe live Redis and Dragonfly: after Allow, the bucket key's `PTTL` is in `(0, ttl]`.
- Two self-contained test files named in the brief. Helpers suffixed `13704`.

**Non-Goals:**

- Product edits in `memory.go`, `lua.go`, `redis.go`.
- A `PTTL` requirement on `std_go_tokenbucket_lua-eval`.
- New debt naming Memory `dropExpired`/`dropOne` (row already Taken: `makeRoomLocked` plus existing benchmarks).
- Shared helper files, copying other issues' tests, extra `*_yaegi_test.go` for #13704.
- Changing `TestLive_Limiter` or the 65536 cap tests.

## Decisions

1. **Files and names.** `tokenbucket/traefik_issue13704_test.go` (`TestTraefik13704_MemoryReclaimsExpiredBuckets`) and `tokenbucket/traefik_issue13704_e2e_test.go` (`TestTraefik13704_RedisBucketExpires`). E2e helpers `liveKey13704` and `evalKey13704` in that e2e file only. Alternative: one combined file or helpers in `limiter_e2e_test.go` — rejected; brief names two files and forbids shared helpers that would collide with parallel PRs.

2. **Memory clock.** `SetNowForTest` at a frozen Unix instant, Allow ten synthetic keys, jump `ttl+9s`, Allow `"fresh"`, then `len(limiter.buckets) == 1` under the mutex. Call `Allow` as `(status, err)` and fail on `err != nil`. Do not sleep wall `ttl`. Alternative: wait real time — rejected; same as the reference and `memory_test.go`.

3. **Live PTTL.** Reuse `runForEachLiveEngine` / `waitLiveClient` with `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`. After three Allows, `evalKey13704` runs `pttl` on `KEYS[1]` via `simpleredis.ScriptSHA1Hex` (Dragonfly requires the key in `KEYS`). Parse milliseconds; assert `(0, ttl]`. Same assertion on both engines. `simpleredis` does not export `Pttl`. Alternative: add a `PTTL` SHALL — rejected; integer `EXPIRE` is already the contract. Alternative: copy `liveKey`/`evalKey` from the repro tree — rejected; suffix `13704`.

4. **Spec fold only.** One scenario under the existing Memory cap/expiry requirement. Do not write a `std_go_tokenbucket_lua-eval` delta. Alternative: skip all spec folders — rejected; the ten-plus-one case is a live promise the 65536 scenarios do not show.

5. **No Memory `dropExpired`/`dropOne` follow-up.** Dest replaced those with `makeRoomLocked` and already has `BenchmarkMemoryAllow` / `BenchmarkYaegiMemoryAllow`. `issues.md` row is Taken. Do not reopen it or write `knowledge/debt/` that names APIs that are gone.

## Risks / Trade-offs

- [Dragonfly `PTTL` differs from Redis] → Mitigation: same assertion on both via `runForEachLiveEngine`; explore measured 6999 ms on both with `ttl` 7s. A `-1` or `0` fails the test; it is not a product change.
- [Parallel issue PRs declare the same helper] → Mitigation: helpers stay in this e2e file with suffix `13704`.
- [White-box `len(limiter.buckets)` couples the test to the map] → Mitigation: in-package tests already read `buckets` (`memory_test.go`); the brief requires counting stored sources. Keep the mutex around the read.

## Migration Plan

Tests only. Rollback is revert. No data migration.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
