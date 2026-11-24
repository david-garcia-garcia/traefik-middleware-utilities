## Purpose

Per-key in-memory admission: Allow decides whether a real backend attempt may proceed, Report records that attempt's boolean outcome on a saturating success-credit bucket, and idle keys expire. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Allow maps admit and retryAfter
`Allow(key)` SHALL return a `Permit`, a `retryAfter` duration, and an error for the opaque `key`. `Permit.Allowed()` SHALL report whether a backend attempt is admitted. A denied Allow SHALL return the zero `Permit`, whose `Allowed()` is false. An admitted Allow SHALL return a permit that identifies that attempt's key and the key's current generation. The caller SHALL NOT invoke Allow when the request context is already canceled or past deadline; the library MUST NOT read `context.Context` on the Allow path. When admitted, `retryAfter` SHALL be zero. When denied, `retryAfter` SHALL be the remaining OPEN cooldown, or the remaining HALF-OPEN probe lease when a probe is outstanding. The library MUST NOT sleep and MUST NOT write HTTP. Prefixing of `key` is the caller's job. Denied requests MUST NOT be treated as backend attempts. Allow MUST NOT return `context.Canceled` or `context.DeadlineExceeded`.

#### Scenario: Closed key admits
- **WHEN** a key is CLOSED with credit above zero
- **AND** Allow is called
- **THEN** the returned permit's `Allowed()` is true
- **AND** retryAfter is zero
- **AND** err is nil

#### Scenario: Denied Allow returns the zero permit
- **WHEN** a key is OPEN and its cooldown has not elapsed
- **AND** Allow is called
- **THEN** the returned permit's `Allowed()` is false
- **AND** retryAfter is the remaining cooldown

#### Scenario: Caller owns cancellation before Allow
- **WHEN** the caller's request context is already done
- **AND** the caller does not invoke Allow
- **THEN** the gate's credit and state for that key do not change on that request path

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Report applies saturating credit
`Report(permit, success)` SHALL record the outcome of a real backend attempt the gate admitted, identified by the permit Allow returned for it. Each key SHALL carry a generation that changes on every state transition and is never reused by a recreated key. Report SHALL apply only when the permit's generation equals the key's current generation; any other permit (zero permit, earlier generation, expired or recreated key) SHALL be ignored without changing credit, state, `n`, or the idle deadline. Report SHALL return `applied` true when the outcome counted for the key and false when it was ignored, so the caller can tell a counted probe result from a stale one; a closed gate SHALL return false and an error. A failure SHALL subtract 1 from that key's credit. A success SHALL add `p/(1-p)` to credit, capped at `B`. Credit SHALL start at `B`. When credit is at most zero while CLOSED, the key SHALL trip to OPEN. Report MUST NOT take a context: a canceled request that already hit the backend MUST still land. Report on OPEN SHALL be ignored.

#### Scenario: Consecutive failures trip a dead backend
- **WHEN** a new key has credit `B`
- **AND** Allow admits `B` attempts
- **AND** Report is called with success false for each of those permits
- **THEN** after the Bth failure the key is OPEN
- **AND** the next Allow is denied

#### Scenario: Success credits toward the ratio
- **WHEN** a CLOSED key has credit below `B`
- **AND** Report is called with success true for a permit of the current generation
- **THEN** credit increases by `p/(1-p)` not above `B`

#### Scenario: Report without context still records
- **WHEN** Allow admitted a request whose context is later done
- **AND** Report is called with that permit and the outcome
- **THEN** credit and state update as for any other Report

#### Scenario: Report from an earlier generation is ignored
- **WHEN** a permit was admitted while the key was CLOSED
- **AND** the key has since tripped and re-closed
- **AND** Report is called with that permit and success false
- **THEN** credit, state, and `n` do not change

#### Scenario: Report says whether the outcome counted
- **WHEN** Report is called with a permit of the key's current generation
- **THEN** it returns applied true
- **WHEN** Report is called with a zero permit, a permit of an earlier generation, or an expired probe's permit
- **THEN** it returns applied false and a nil error

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

### Requirement: Memory map bounds idle keys
Idle keys SHALL expire after `TTL` on a later Allow (lazy). Allow SHALL refresh the key's idle deadline on every call, including denies, so an OPEN key under denied traffic is not dropped. When a key has been idle longer than `TTL`, the next Allow SHALL treat it as unseen (CLOSED, credit `B`, `n` 0). The map SHALL not grow past 65536 keys: expired keys drop first, then one arbitrary slot if still at cap. The package MUST NOT import `tokenbucket`.

#### Scenario: Idle key is presumed healthy
- **WHEN** a key has been OPEN
- **AND** no Allow is called for longer than TTL
- **AND** Allow is called
- **THEN** that Allow returns allowed true
- **AND** credit is `B`

#### Scenario: Deny refreshes idle deadline
- **WHEN** a key is OPEN
- **AND** Allow is called (denied) at intervals shorter than TTL for longer than the cooldown
- **THEN** the key is not dropped
- **AND** after the cooldown elapses Allow may enter HALF-OPEN instead of a fresh CLOSED key

### Requirement: Construction validates and defaults knobs
`New(Config)` SHALL apply defaults for any zero field: `FailureRatio` 0.30, `TripFailures` 5, `BaseCooldown` 1s, `MaxCooldown` 10s, `Jitter` 0.10, `TTL` 60s. Construction SHALL fail when `FailureRatio` is not in (0, 1), `TripFailures` is less than 1, `BaseCooldown` is not greater than zero, `MaxCooldown` is less than `BaseCooldown`, `Jitter` is not in [0, 1), or `TTL` is less than 1s. `Jitter` 0 SHALL be valid and SHALL disable jitter.

#### Scenario: New rejects invalid ratio
- **WHEN** New is called with FailureRatio 1 or a negative value
- **THEN** construction returns an error
- **AND** no map entry is created

#### Scenario: Zero Config uses defaults
- **WHEN** New is called with a zero Config
- **THEN** construction succeeds
- **AND** FailureRatio is 0.30
- **AND** TripFailures is 5

### Requirement: Close releases the map
`Close` SHALL drop stored keys. After Close, Allow and Report SHALL return an error and MUST NOT admit. Close SHALL be safe to call more than once. Sleep and Wake hooks are not required.

#### Scenario: Allow after Close
- **WHEN** Close has been called
- **AND** Allow is called
- **THEN** Allow returns an error
- **AND** allowed is false

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
