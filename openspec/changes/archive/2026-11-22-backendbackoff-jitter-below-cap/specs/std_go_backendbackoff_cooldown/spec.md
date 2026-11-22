# Spec Delta

## MODIFIED Requirements

### Requirement: Cooldown opens and probes
When credit trips, the key SHALL become OPEN and deny Allow for a jittered cooldown. The nominal cooldown SHALL be `BaseCooldown * 2^n`, capped at `MaxCooldown`. With `Jitter` greater than zero, the jittered cooldown SHALL be `lower + u * (upper - lower)` for `u` in [0, 1), where `lower = cooldown * (1 - Jitter)` and `upper = min(cooldown * (1 + Jitter), MaxCooldown)`. The jittered cooldown MUST NOT exceed `MaxCooldown` and MUST NOT be produced by clamping a sample onto `MaxCooldown`. With `Jitter` 0 the cooldown SHALL be the nominal cooldown exactly. When that wait has elapsed, the next Allow SHALL enter HALF-OPEN and admit one probe. At most one probe SHALL be outstanding. Concurrent Allows while a probe is outstanding SHALL deny with `retryAfter` equal to the remaining probe lease. The probe lease SHALL be `BaseCooldown` from HALF-OPEN entry. If that lease elapses with no Report, the next Allow MAY take the probe slot.

#### Scenario: First trip waits the base
- **WHEN** a key trips with `n` 0 and Jitter 0
- **AND** Allow is called immediately
- **THEN** allowed is false
- **AND** retryAfter is BaseCooldown
- **WHEN** the clock advances BaseCooldown
- **AND** Allow is called
- **THEN** allowed is true (the probe)

#### Scenario: Second Allow during probe is denied
- **WHEN** a key is HALF-OPEN with a probe outstanding
- **AND** Allow is called
- **THEN** allowed is false
- **AND** retryAfter is the remaining probe lease

#### Scenario: Jitter at the cap has no point mass
- **WHEN** the nominal cooldown equals MaxCooldown
- **AND** Jitter is 0.1
- **AND** many cooldowns are drawn
- **THEN** every cooldown is at least `0.9 * MaxCooldown` and below MaxCooldown
- **AND** no single cooldown value accounts for a material share of the draws

#### Scenario: Jitter below the cap is unchanged
- **WHEN** `cooldown * (1 + Jitter)` does not exceed MaxCooldown
- **THEN** cooldowns lie in `[cooldown * (1 - Jitter), cooldown * (1 + Jitter))`
