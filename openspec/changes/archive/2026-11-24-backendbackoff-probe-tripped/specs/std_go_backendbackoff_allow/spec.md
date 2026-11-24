# Spec Delta

## ADDED Requirements

### Requirement: Probe and Tripped expose the backoff state
`Permit.Probe()` SHALL report whether Allow admitted that attempt as the HALF-OPEN probe of a tripped key: true for the admit that moves OPEN to HALF-OPEN and for an admit that takes over an expired probe lease, false for every CLOSED admit (including the first admit after an idle key was dropped) and for the zero `Permit`. `Tripped(key)` SHALL report whether the key is OPEN or HALF-OPEN. A key that is not stored, or whose idle TTL has elapsed, SHALL NOT be tripped. Tripped MUST NOT create the key, change its state or credit, or refresh its idle deadline. After Close, Tripped SHALL return false and an error. A caller that logs backoff transitions SHALL be able to tell, from these two values alone, a probe that closed the key from a key the gate dropped after TTL.

#### Scenario: Tripped follows trip, probe and close
- **WHEN** a key trips to OPEN
- **THEN** Tripped returns true, also once the cooldown has elapsed and while the probe is outstanding
- **WHEN** the probe's Report with success true applies
- **THEN** Tripped returns false

#### Scenario: Tripped does not keep an idle key
- **WHEN** a key is OPEN
- **AND** no Allow or applied Report touches it for TTL
- **THEN** Tripped returns false, even when Tripped was called before TTL elapsed
- **AND** the next Allow returns a permit whose `Probe()` is false

#### Scenario: Only HALF-OPEN admits are probes
- **WHEN** Allow admits while the key is CLOSED
- **THEN** the permit's `Probe()` is false
- **WHEN** Allow admits after the OPEN cooldown, or after an outstanding probe's lease expired
- **THEN** the permit's `Probe()` is true

## MODIFIED Requirements

### Requirement: Unit and Yaegi prove Allow
Compiled tests SHALL prove consecutive-failure trip, success credit, idle drop, deny refreshes TTL, that a report from an earlier generation is ignored, and what Probe and Tripped report, without Traefik. Interpreted tests SHALL run an Allow-then-Report trip, and a trip followed by Tripped and a probe permit, with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time. `TestAlloc*` SHALL fail when the warm Allow path (existing key, CLOSED) exceeds the recorded allocs/op and B/op ceilings, and SHALL skip under the race detector.

#### Scenario: Yaegi Tripped then probe
- **WHEN** the Yaegi GOPATH interp loads non-test `backendbackoff` sources
- **AND** the probe trips a key, then Allows after the cooldown and Reports success
- **THEN** Tripped is true after the trip and false after the probe
- **AND** only the admit after the cooldown has `Probe()` true

#### Scenario: Yaegi Allow then Report trips
- **WHEN** the Yaegi GOPATH interp loads non-test `backendbackoff` sources
- **AND** the probe Allows and Reports failures `B` times with the permits Allow returned
- **THEN** the next Allow is denied
- **AND** the interp MUST NOT enable unsafe
