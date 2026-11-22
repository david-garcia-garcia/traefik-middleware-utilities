# Spec Delta

## MODIFIED Requirements

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
