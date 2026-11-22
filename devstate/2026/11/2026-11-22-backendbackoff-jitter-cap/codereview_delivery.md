# Delivery

## Motivation

After a trip, the backendbackoff gate waits `BaseCooldown * 2^n` (already capped at `MaxCooldown`) and then jitters that wait so replica probes do not land together. `Jitter` 0 freezes the wait; a positive `Jitter` is the spread.

The spread was `wait * (1 + J * (2u - 1))` with `u` in `[0, 1)`, then a clamp onto `MaxCooldown`. Once `wait` is already the cap, every draw with `u > 0.5` overshoots and returns exactly `MaxCooldown`: 50.2% of 100,000 at-cap samples piled on that one Duration. The same clamp fires when `wait` is still below the cap but `wait * (1 + J)` is not: with `BaseCooldown` 9.5s, `MaxCooldown` 10s, `Jitter` 0.1, 23.9% of first-trip waits landed on the max.

Left alone, a long shared outage drives every replica to the cap. R replicas and K keys then send about `R*K/2` probes on the same instant, each cycle, at the backend that is trying to recover. That is the opposite of what `Jitter` is for.

Priority: P1 — production replica probes lockstep at MaxCooldown today, a wrong jitter contract

## Implementation

`cooldownDuration` still doubles `BaseCooldown` up to `MaxCooldown`. When `Jitter` is greater than zero it now builds the interval first — `lower = wait * (1 - J)`, `upper = min(wait * (1 + J), MaxCooldown)` — and draws uniformly inside it (`lower + u * (upper - lower)`, `u` in `[0, 1)`). There is no clamp after the sample. When the +J side already fits under the cap, the interval is the old ±J band. When `upper` is the cap, the half-open draw stays below `MaxCooldown`, so the cap is an endpoint of the range, not a pile-up. `Jitter` 0 still returns `wait` exactly. The cooldown spec states that interval; proofs cover at-cap, near-cap, below-cap, and the zero-jitter control.

## What this changes
**Operators.** Replica probes at a capped cooldown spread instead of landing together on MaxCooldown; existing Jitter and MaxCooldown knobs, no new deploy key.
**Admin users.** None.
**Developers.** Jittered cooldown is uniform on `[wait*(1-J), min(wait*(1+J), MaxCooldown))`; Jitter 0 is still exact wait; Allow/Report call shape is unchanged.
**End users.** None.
