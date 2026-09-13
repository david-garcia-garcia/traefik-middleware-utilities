## Context

Dest `Memory.Allow` and `Redis.Allow` return `(bool, time.Duration, error)`. Refund is already `waitMicro > float64(maxDelay.Microseconds())` in `consumeOne` and Traefik Lua (`wait_duration > max_delay`, ARGV whole microseconds). Admit on dest is `waitDuration(waitMicro) <= maxDelay`. Specs: `std_go_tokenbucket_allow`. Explore: `devstate/2026/09/2026-09-13-tokenbucket-bug-allow-wait-units/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- One microseconds comparison for admit and refund on both stores.
- Keep Allow arity and Duration return; delete `allowedFromWait` and named `waitDuration`.
- Tests that fail on dest, then pass after the how.

**Non-Goals:**
- Changing Lua or ARGV encoding.
- Rounding maxDelay up, skipping refund, or special-casing 1500ns.
- Other tokenbucket bugs (Eval nan/Inf wait).
- Dropping the Duration return (`std_go_tokenbucket_allow`).

## Decisions

1. **Admit is `waitMicro <= float64(maxDelay.Microseconds())`.** Same units Lua already uses. Alternative: round maxDelay up to the next microsecond — rejected; agreed how forbids it. Alternative: skip refund when Duration would still admit — rejected; papers over the split.

2. **Redis.Allow uses the already-parsed `waitMicro` for the bool.** Do not convert to `time.Duration` to decide allowed. Alternative: keep `allowedFromWait(wait, maxDelay)` — rejected; that is the dest fail-open.

3. **Keep `(bool, time.Duration, error)`.** Inline `time.Duration(waitMicro * float64(time.Microsecond))` (with the existing `waitMicro <= 0 → 0` guard) at the two Allow returns. Alternative: drop Duration — rejected; dest spec and enumerated callers still return wait. Alternative: keep named `waitDuration` — rejected; agreed how deletes it once it is not the admit conversion.

4. **Leave `tokenbucket/lua.go` and ARGV[5] as `maxDelay.Microseconds()`.** Go maps allowed. Alternative: change Lua to return a deny flag — rejected; first Lua field is always `"true"` on Traefik’s script.

5. **Tests first, then production.** Package tests adapted from caller repros: 1500ns + rate `1e6/1.2`; 1µs + rate 999999; maxDelay 0 + rate `1e12`; rate `1e-12` overflow. Adapt `Allow(ctx, key)` arity; do not weaken. Alternative: implement first then add tests — rejected; caller required dest-fail then pass.

## Risks / Trade-offs

- [Wait Duration can still overflow or truncate while allowed is correct] → Mitigation: admit does not use that Duration; existing tests that read wait keep the second return.
- [Redis live path not covered by `-short`] → Mitigation: same Go predicate as memory; fake-TCP Redis tests already share Allow mapping.
- [Usage packet still says Go maps allowed from wait vs maxDelay] → Mitigation: update the gotcha in the same change.

## Migration Plan

Library callers keep the same Allow signature. Rollback is revert. No store rewrite.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
