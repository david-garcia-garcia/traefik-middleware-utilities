## Purpose

Per-key in-memory admission: Allow decides whether a real backend attempt may proceed, Report records that attempt's boolean outcome on a saturating success-credit bucket, and idle keys expire. Callers own identity; the library never reads HTTP.

## ADDED Requirements

### Requirement: Allow maps admit and retryAfter
`Allow(ctx, key)` SHALL return whether a backend attempt for the opaque `key` is admitted, a `retryAfter` duration, and an error. A caller with no deadline SHALL pass `context.Background()`. When `ctx` is already done, Allow SHALL return that error and MUST NOT admit. When admitted, `retryAfter` SHALL be zero. When denied, `retryAfter` SHALL be the remaining OPEN cooldown, or the remaining HALF-OPEN probe lease when a probe is outstanding. The library MUST NOT sleep and MUST NOT write HTTP. Prefixing of `key` is the caller's job. Denied requests MUST NOT be treated as backend attempts.

#### Scenario: Closed key admits
- **WHEN** a key is CLOSED with credit above zero
- **AND** Allow is called
- **THEN** Allow returns allowed true
- **AND** retryAfter is zero
- **AND** err is nil

#### Scenario: Canceled context does not admit
- **WHEN** ctx is already done
- **AND** Allow is called
- **THEN** Allow returns that context error
- **AND** allowed is false
- **AND** the key's credit and state do not change

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Report applies saturating credit
`Report(key, success)` SHALL record the outcome of a real backend attempt the gate admitted. A failure SHALL subtract 1 from that key's credit. A success SHALL add `p/(1-p)` to credit, capped at `B`. Credit SHALL start at `B`. When credit is at most zero while CLOSED, the key SHALL trip to OPEN. Report MUST NOT take a context: a canceled request that already hit the backend MUST still land. Report on OPEN with no outstanding probe SHALL be ignored. Denied Allows MUST NOT be Reported by the caller; if they are, the library SHALL ignore them the same way.

#### Scenario: Consecutive failures trip a dead backend
- **WHEN** a new key has credit `B`
- **AND** Allow admits
- **AND** Report is called with success false, `B` times
- **THEN** after the Bth failure the key is OPEN
- **AND** the next Allow returns allowed false

#### Scenario: Success credits toward the ratio
- **WHEN** a CLOSED key has credit below `B`
- **AND** Report is called with success true
- **THEN** credit increases by `p/(1-p)` not above `B`

#### Scenario: Report without context still records
- **WHEN** Allow admitted a request whose context is later done
- **AND** Report is called with the outcome
- **THEN** credit and state update as for any other Report

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
- **WHEN** New is called with FailureRatio 0 or 1 or a negative value
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
Compiled tests SHALL prove consecutive-failure trip, success credit, idle drop, deny refreshes TTL, and canceled context, without Traefik. Interpreted tests SHALL run an Allow-then-Report trip with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time. `TestAlloc*` SHALL fail when the warm Allow path (existing key, CLOSED) exceeds the recorded allocs/op and B/op ceilings, and SHALL skip under the race detector.

#### Scenario: Yaegi Allow then Report trips
- **WHEN** the Yaegi GOPATH interp loads non-test `backendbackoff` sources
- **AND** the probe Allows and Reports failures `B` times
- **THEN** the next Allow is denied
- **AND** the interp MUST NOT enable unsafe
