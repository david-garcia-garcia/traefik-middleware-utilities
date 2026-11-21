## Context

Dest `tokenbucket/` already has `consumeOne`, Memory `Allow`, and Lua `allowScript` (Traefik MIT copy). See proposal.md for why. Specs: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`. Explore decisions: `devstate/explore.md`. Fake Redis EVAL applies `consumeOne`, not a Lua VM.

## Goals / Non-Goals

**Goals:**
- Never persist a `last` in the past on Memory or Redis EVAL.
- Keep the elapsed backward clamp (no negative refill).
- Memory lock order is clock order.
- Tests fail on dest, then pass after the fix.

**Non-Goals:**
- Idle-fill-to-burst, wait Duration mapping, NaN/Inf rate, TTL truncation, cap eviction.
- Dropping the elapsed clamp.
- Fixing only the goroutine path.
- Interpreting Lua in the fake Redis.

## Decisions

1. **Tests first in `tokenbucket/repro_hunt_clock_last_backward_test.go`.** Copy/adapt `TestRepro_StaleNowRewindsLastDoubleRefill` with dest `Allow(ctx, key) (bool, time.Duration, error)`. Inline `newBurst1Delay0` / `mustAllow`; reuse package `testTTL`. Alternative: fold into `limiter_test.go` — rejected; Desired names that hunt file.

2. **`consumeOne` persist-max.** Capture previous last before the elapsed clamp; return `max(previous last, nowMicro)`. Alternative: persist clamped last (always now when backward) — rejected; that is dest today. Alternative: drop the clamp — rejected; negative elapsed.

3. **Lua sibling two-step.** Keep `if t < last then last = t end` for elapsed; HSET last as previous `bucket.last` unless `t` is greater. Keep MIT notice. Alternative: leave Lua bit-identical — rejected; ticket names HSET. Alternative: `math.max` only — equivalent; either form is persist-max.

4. **Memory now after `mu.Lock()`.** `expireAt` uses that same now. Alternative: only persist-max in `consumeOne` — rejected; a stale sample after a newer last still writes an earlier now unless persist-max also holds, and lock-order clock is named in Desired. Persist-max still required so sequential rewind is fixed even without concurrency.

5. **Script assertion for Lua.** Fake Redis cannot fail a stale `lua.go`. Assert `allowScript` HSET last is persist-max, not raw `t`. No extra Redis Allow sequence.

## Risks / Trade-offs

- [Diverge from Traefik Lua last field] → Mitigation: ticket wins; keep MIT notice; spec records persist-max.
- [TTL expireAt uses a later now after moving the sample inside the lock] → Mitigation: same Allow still sets expireAt from the consume's now; ttl behavior unchanged for sequential callers.
- [Fake Redis stays on consumeOne] → Mitigation: script-text assertion for HSET; live e2e already agrees Memory vs Redis on other sequences.

## Migration Plan

Library bugfix. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
