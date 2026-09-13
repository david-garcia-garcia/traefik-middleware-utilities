## Context

DestBranch `tokenbucket/` already has Memory, Redis Eval of copied Traefik Lua, `consumeOne`, and a fake TCP Redis. See proposal.md for why. Specs: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`. Explore: `devstate/explore.md`. Caller identity is not reconstructed; Allow key stays caller-owned.

## Goals / Non-Goals

**Goals:**
- Missing state is a full bucket, then consume 1, on Memory and Redis.
- Tests fail on dest first (`epoch_clock`, `huge_burst_elapsed_below_burst`), then pass.
- Fake Redis missing-hash seed matches Lua so agreement still holds under `-short`.

**Non-Goals:**
- Wait mapping, NaN/Inf rate, ttl seconds truncation, last rewind, source-cap eviction.
- Importing `golang.org/x/time/rate`.
- Changing `Allow(ctx, key)` signature.
- Live Redis/Dragonfly as the only proof.

## Decisions

1. **Seed full, then consumeOne.** Memory new `memEntry` (miss and TTL delete) sets `tokens = burst`, `last = nowMicro` before `consumeOne`. Alternative: special-case `last=0` inside `consumeOne` — rejected; ticket keeps consume math as consume.

2. **Lua empty hash, not last=0.** When `#rl_source ~= 4`, set `tokens = burst` and `last = t`. Keep MIT notice. Alternative: leave Traefik `tokens=0` `last=0` — rejected; fails at epoch and huge burst.

3. **Fake missing hash like Lua.** Missing hash seeds `tokens=burst`, `last=now` then `consumeOne`. Alternative: change `consumeOne` for zeros — rejected; Memory never calls it with zeros after the Memory seed.

4. **Test file.** `tokenbucket/repro_epoch_idle_burst_test.go` `TestRepro_NewKeyFillsToBurstAtEpoch` with dest `Allow(ctx, key)`. Redis/fake same two cases plus `allowScript` contains the empty-hash seed. No extra epoch TTL subtest (one `memEntry` constructor).

5. **No x/time/rate.** Memory stays Lua formulas.

## Risks / Trade-offs

- [Copied Traefik Lua diverges on empty hash] → Mitigation: delta spec + MIT notice kept; research notes still describe Traefik `last=0`.
- [Fake is consumeOne, not Lua] → Mitigation: seed like Lua; assert script text for the empty-hash branch; agreement test still runs.
- [Existing burst-after-idle tests freeze far from epoch] → Mitigation: new epoch and huge-burst cases; leave those tests as regression.

## Migration Plan

Library behavior change on first Allow of a missing key. No schema migration. Rollback is revert. Redis hashes already stored keep their last/tokens; only missing hashes change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
