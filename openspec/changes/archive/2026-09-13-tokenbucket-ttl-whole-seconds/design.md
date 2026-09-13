## Context

Dest `validateClock` rejects only `ttl < time.Second`. Memory `expireAt = now.Add(ttl)`. Redis ARGV is `ttlSeconds()` (`int64(ttl / time.Second)`); Lua `redis.call('expire', key, ttl)`. Fake Redis does not EXPIRE; Redis lifetime on unit tests is ARGV, not waiting for the hash to drop. Proceed policies: `devstate/explore.md`. See proposal.md for why. Specs: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`.

## Goals / Non-Goals

**Goals:**
- One `validateClock` check so both constructors reject a ttl Redis cannot pass as EXPIRE seconds.
- Fail-then-pass tests: dest accepts 1500ms / ARGV `"1"` / Memory live at +1200ms; after the gate, New(1500ms) is `errTTL` and 2s still constructs.

**Non-Goals:**
- PEXPIRE or millisecond Redis expiry.
- Lua / Traefik `expire` rewrite.
- Flooring Memory `expireAt` to `ttlSeconds` while New(1500ms) succeeds.
- Other `tokenbucket` bugs (maxDelay, Eval NaN, clock last, cap eviction).

## Decisions

1. **Whole-second check on `validateClock`, not a Redis-only branch.** Both stores share the clock. Alternative: floor Memory to `ttlSeconds` — rejected; Desired forbids silent floor while New succeeds. Alternative: PEXPIRE — rejected.

2. **Same `errTTL` sentinel; text names whole seconds.** `ttl < time.Second || ttl%time.Second != 0` returns `errTTL` with text `tokenbucket: ttl must be a whole number of seconds (at least 1s)`. Alternative: a second error value — rejected; Desired keeps the sentinel.

3. **Tests first, then reshape the same file.** Land `tokenbucket/ttl_truncation_test.go` adapted from the caller repro (`Allow(ctx, key)` three-value return). First commit: dest-failing (New accepts 1500ms, ARGV `"1"`, Memory not expired at +1200ms). Then the gate. Then reshape assertions to `errors.Is(..., errTTL)` for 1500ms on both constructors and keep 2s accepted. Alternative: keep ARGV/`+1200ms` as post-fix contract — rejected; Desired says reshape to fractional ttl rejected.

4. **EVAL ARGV index 6 is the dest fake’s ttl slot.** `EVAL` script numkeys key limit burst TTL now maxdelay. Fake does not run EXPIRE. Alternative: live Redis TTL wait — not needed for the constructor gate.

## Risks / Trade-offs

- [Callers that passed 1500ms now fail New] → Mitigation: **BREAKING** on the constructor; Traefik itself uses whole-second ttl (`ext_traefik_ratelimiter_token-bucket`).
- [Dest-failing test must not be weakened to pass] → Mitigation: measure FAIL on dest before editing `validateClock`.
- [`errTTL` string change breaks string-matched callers] → Mitigation: keep the sentinel; tests use `errors.Is`.

## Migration Plan

Library constructor tightening. Rollback is revert. No Redis data migration.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
