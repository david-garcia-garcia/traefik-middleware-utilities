## Context

See proposal.md for why. `validateClock` already rejects a caller ttl below 1s or with a fractional second, and both stores read `clockConfig.ttl` (`memory.go` `now.Add`, `redis.go` `ttlSeconds` into Lua `EXPIRE`). An expired or missing key is seeded at `burst`. Proceed policies are on `devstate/explore.md`. Measured: rate 0.5, burst 30, ttl 3s, a 5s pause admits 30.

## Goals / Non-Goals

**Goals:**

- One effective ttl on `clockConfig.ttl`, applied after `validateClock`, shared by Memory and Redis.
- Floor `ceil(burst/rate + maxDelay.Seconds())` whole seconds, never below the caller ttl, clamped to the largest whole-second `time.Duration`.
- The #13957 repros pass (Memory admits 2; live admits at most 3) and the control case stays.

**Non-Goals:**

- Rejecting a short but valid caller ttl.
- Changing `consumeOne`, the Lua seed, or `EXPIRE`'s integer-second unit.
- PEXPIRE, or flooring a fractional ttl while `New` succeeds (that remains `errTTL`).

## Decisions

1. **Raise after validation, store one field.** `effectiveTTL` runs only after `validateClock` returns nil, and its result is what `clockConfig.ttl` holds. Memory and Redis do not grow a second ttl field. Alternative: reject in `validateClock` — rejected; callers that pass Traefik's formula would break. Alternative: raise only in Redis `EXPIRE` — rejected; the stores would disagree (the whole-second invariant).

2. **Clamp with integer seconds, not a float times `time.Second`.** The cap is `(math.MaxInt64 / int64(time.Second)) * time.Second` (9223372036s). `math.Ceil` of `burst/rate + maxDelay.Seconds()` is taken in float, then clamped to that integer before converting. Alternative: let `time.Duration` wrap — rejected; a wrapped ttl expires immediately and recreates a full bucket.

3. **The truncation test's ARGV `"2"` becomes `"3"`.** `NewRedis(rate=1, burst=2, maxDelay=1µs, ttl=2s)` floors to 3s. The 1500ms rejection stays. No other test compares ARGV ttl to the caller value.

4. **Repros are new files, not edits of the shared upstream file.** Helpers are suffixed `13957`. `Allow` is `Status`; an admit is `Status.Allowed`. Live tests use `runForEachLiveEngine` and `waitLiveClient` and skip under `-short` or without the env vars.

## Risks / Trade-offs

- [Idle keys live longer, so Memory holds more sources and Redis keeps more hashes] → Mitigation: the floor is the time the bucket needs to be full anyway; the 65536 cap still evicts.
- [`TestYaegi_MemoryCapAndExpiry` uses rate 1e-6 and ttl 1s, so the floor is 1000000s and a 1s step no longer expires] → Mitigation: the probe only requires the new key to be admitted; cap eviction still does that. Retune the probe only if the test fails.
- [Downstream importers are not in this repo] → Mitigation: the signature is unchanged; a ttl that was already at least the floor behaves as before.

## Migration Plan

Library behavior only. Rollback is revert. No stored schema change: existing Redis hashes keep their remaining `EXPIRE`, and the next `Allow` sets the new seconds.

## Open Questions

None. The floor, the clamp, and the ARGV assertion are decided on `devstate/explore.md`.
