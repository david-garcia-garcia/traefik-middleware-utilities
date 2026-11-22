## Why

`cooldownDuration` jitters as `wait × (1 + J × (2u − 1))` and then clamps to `MaxCooldown`. Once `wait` is at the cap, every sample with `u > 0.5` overshoots and is clamped to exactly the max: measured 50.2 % of 100,000 samples on one value. Near the cap the same clamp leaves a smaller point mass (23.9 % with `Base` 9.5s, `Max` 10s, `J` 0.1, `n` 0). The code comment says jitter exists "so replicas that trip together do not probe in lockstep"; during a long shared outage half of the replicas still probe on the same instant.

## What Changes

- The jittered cooldown is drawn uniformly from `[lower, upper)` with `lower = wait × (1 − J)` and `upper = min(wait × (1 + J), MaxCooldown)`, where `wait` is the nominal `BaseCooldown × 2^n` already capped at `MaxCooldown`. No clamp after sampling.
- When `wait × (1 + J) ≤ MaxCooldown` the interval and distribution are exactly today's. Only the overshooting band changes: it shrinks to end at the cap instead of piling onto it.
- `Jitter` 0 still returns `wait` exactly.
- Spec `std_go_backendbackoff_cooldown` states the new formula.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_backendbackoff_cooldown`: jitter is sampled inside `[wait(1−J), min(wait(1+J), MaxCooldown))`; no point mass at the cap.

## Impact

- `backendbackoff/gate.go` `cooldownDuration` only. `allow.go` is not touched (siblings own it).
- New `backendbackoff/zzz_jitter_below_cap_test.go`. Existing tests use `Jitter: 0` and keep passing.
- Sibling tickets edit `allow.go` and `memEntry`; this hunk is confined to `cooldownDuration`.
