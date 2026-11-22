# Spec Delta

## MODIFIED Requirements

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

### Requirement: Unit and Yaegi prove Allow
Compiled tests SHALL prove consecutive-failure trip, success credit, idle drop, deny refreshes TTL, and that a report from an earlier generation is ignored, without Traefik. Interpreted tests SHALL run an Allow-then-Report trip with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time. `TestAlloc*` SHALL fail when the warm Allow path (existing key, CLOSED) exceeds the recorded allocs/op and B/op ceilings, and SHALL skip under the race detector.

#### Scenario: Yaegi Allow then Report trips
- **WHEN** the Yaegi GOPATH interp loads non-test `backendbackoff` sources
- **AND** the probe Allows and Reports failures `B` times with the permits Allow returned
- **THEN** the next Allow is denied
- **AND** the interp MUST NOT enable unsafe
