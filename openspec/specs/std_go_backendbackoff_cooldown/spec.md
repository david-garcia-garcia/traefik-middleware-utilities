## Purpose

Exponential cooldown that decides when an OPEN key may probe again, independent of credit: jittered wait, one HALF-OPEN probe, retained backoff exponent, and reset after a long healthy closed stretch.

## Requirements

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

### Requirement: Probe outcome retains or increments n
Only the report whose permit is the active probe's permit SHALL decide the HALF-OPEN outcome. A successful probe Report SHALL close the key, restore credit to `B`, and retain `n`. A failed probe Report SHALL re-open with `n+1` regardless of credit. A report whose permit was issued before the probe (an earlier CLOSED era) or to a probe whose lease expired and was replaced SHALL be ignored. Cooldown math MUST NOT use Report outcomes other than that probe result. Credit arithmetic MUST NOT use elapsed time.

#### Scenario: Probe success keeps n
- **WHEN** a key is HALF-OPEN with `n` 1
- **AND** Report is called with the probe's permit and success true
- **THEN** the key is CLOSED
- **AND** credit is `B`
- **AND** a later trip uses `n` 1 (cooldown BaseCooldown * 2, before cap)

#### Scenario: Probe failure increments n
- **WHEN** a key is HALF-OPEN with `n` 0
- **AND** Report is called with the probe's permit and success false
- **THEN** the key is OPEN
- **AND** the next cooldown uses `n` 1

#### Scenario: Stale success does not decide the probe
- **WHEN** a permit was admitted while the key was CLOSED and is still in flight
- **AND** the key trips and a probe is admitted
- **AND** Report is called with the earlier permit and success true
- **AND** Report is then called with the probe's permit and success false
- **THEN** the key is OPEN
- **AND** the next cooldown uses `n+1`

#### Scenario: Late report of an expired probe is ignored
- **WHEN** probe A was admitted and its lease elapsed with no Report
- **AND** a later Allow admitted probe B
- **AND** Report is called with A's permit
- **THEN** the key stays HALF-OPEN with B outstanding
- **AND** B's Report decides the outcome

### Requirement: n resets after a long closed stretch
`n` SHALL reset to 0 once the key has been continuously CLOSED for one `MaxCooldown`. Time spent OPEN or HALF-OPEN SHALL NOT count toward that stretch. A new trip during the stretch SHALL keep `n`.

#### Scenario: Long healthy closed resets n
- **WHEN** a key has `n` 2 and is CLOSED
- **AND** the clock advances MaxCooldown while it stays CLOSED
- **AND** the key later trips
- **THEN** that OPEN cooldown uses `n` 0

#### Scenario: Early re-trip keeps n
- **WHEN** a key has `n` 2 and is CLOSED
- **AND** it trips before MaxCooldown of continuous CLOSED
- **THEN** that OPEN cooldown still uses `n` 2
