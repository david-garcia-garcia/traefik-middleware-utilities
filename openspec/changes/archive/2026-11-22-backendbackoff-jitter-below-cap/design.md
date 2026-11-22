## Context

See proposal.md. Explore listed: (1) uniform in `[Max(1−J), Max]` only at the cap, (2) pre-bounded interval, (3) reject-and-resample, (4) keep the clamp. (4) fails Desired. (1) leaves the near-cap point mass. (3) changes the distribution of every sample that overshoots and costs a loop.

## Goals / Non-Goals

**Goals:**

- Never exceed `MaxCooldown`, no point mass at the cap or near it, `Jitter` 0 exact.
- Unchanged distribution whenever the old formula did not overshoot.

**Non-Goals:**

- Changing `BaseCooldown`, `MaxCooldown`, `n`, the probe lease, or the RNG.
- Full or decorrelated jitter schemes.

## Decisions

1. **Pre-bounded interval (option 2).** In `cooldownDuration`, after computing the capped `wait`: `if J <= 0 { return wait }`; `lower := float64(wait) * (1 - J)`; `upper := math.Min(float64(wait)*(1+J), float64(MaxCooldown))`; `return time.Duration(lower + u*(upper-lower))` with `u = g.rng.Float64()` in `[0,1)`. `upper > lower` always holds because `J > 0` and `wait > 0`. Result is `< MaxCooldown` when `upper` is the cap, `≥ lower > 0` otherwise. Remove the `jittered < 0` and `jittered > Max` branches (unreachable). Alternative (1) — rejected, leaves the 23.9 % near-cap mass. Alternative (3) — rejected, a resample loop under the gate mutex with no benefit over (2).

2. **Spec wording.** The cooldown requirement replaces "`cooldown * (1 + Jitter * (2u - 1))` … capped at `MaxCooldown`" with the interval formula and adds a scenario that no value at the cap carries a point mass.

3. **Tests.** `backendbackoff/zzz_jitter_below_cap_test.go`, helpers suffixed `JitterBelowCap`, seeded RNG via the gate's `rng` field (white-box, same package): (a) at the cap (`n` large), 100,000 samples are all `< MaxCooldown`, all `≥ 0.9 × Max`, and no single `time.Duration` value holds more than 0.1 % of samples; (b) near the cap (`Base` 9.5s, `Max` 10s, `J` 0.1, `n` 0) same properties with `lower = 8.55s`; (c) below the cap (`Base` 1s, `n` 0) min/max of samples lie in `[0.9s, 1.1s)` as before; (d) `Jitter` 0 returns `wait` exactly at and below the cap.

## Risks / Trade-offs

- [Mean cooldown at the cap drops from ~0.975 Max to 0.95 Max] → Within the configured jitter band; probes come slightly earlier on average, never later than Max.

## Migration Plan

Behavior-only change in the jitter distribution. Rollback is revert.

## Open Questions

None.
