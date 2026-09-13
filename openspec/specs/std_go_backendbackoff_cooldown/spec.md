## Purpose

Exponential cooldown that decides when an OPEN key may probe again, independent of credit: jittered wait, one HALF-OPEN probe, retained backoff exponent, and reset after a long healthy closed stretch.

## Requirements

### Requirement: Cooldown opens and probes
When credit trips, the key SHALL become OPEN and deny Allow for `BaseCooldown * 2^n` plus jitter, capped at `MaxCooldown`. Jitter SHALL be `cooldown * (1 + Jitter * (2u - 1))` for `u` in [0, 1). When that wait has elapsed, the next Allow SHALL enter HALF-OPEN and admit one probe. At most one probe SHALL be outstanding. Concurrent Allows while a probe is outstanding SHALL deny with `retryAfter` equal to the remaining probe lease. The probe lease SHALL be `BaseCooldown` from HALF-OPEN entry. If that lease elapses with no Report, the next Allow MAY take the probe slot.

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

### Requirement: Probe outcome retains or increments n
A successful probe Report SHALL close the key, restore credit to `B`, and retain `n`. A failed probe Report SHALL re-open with `n+1` regardless of credit. Cooldown math MUST NOT use Report outcomes other than that probe result. Credit arithmetic MUST NOT use elapsed time.

#### Scenario: Probe success keeps n
- **WHEN** a key is HALF-OPEN with `n` 1
- **AND** Report is called with success true
- **THEN** the key is CLOSED
- **AND** credit is `B`
- **AND** a later trip uses `n` 1 (cooldown BaseCooldown * 2, before cap)

#### Scenario: Probe failure increments n
- **WHEN** a key is HALF-OPEN with `n` 0
- **AND** Report is called with success false
- **THEN** the key is OPEN
- **AND** the next cooldown uses `n` 1

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
