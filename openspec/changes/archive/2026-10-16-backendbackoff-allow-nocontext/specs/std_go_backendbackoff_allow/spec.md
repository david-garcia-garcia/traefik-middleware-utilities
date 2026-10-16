## MODIFIED Requirements

### Requirement: Allow maps admit and retryAfter
`Allow(key)` SHALL return whether a backend attempt for the opaque `key` is admitted, a `retryAfter` duration, and an error. The caller SHALL NOT invoke Allow when the request context is already canceled or past deadline; the library MUST NOT read `context.Context` on the Allow path. When admitted, `retryAfter` SHALL be zero. When denied, `retryAfter` SHALL be the remaining OPEN cooldown, or the remaining HALF-OPEN probe lease when a probe is outstanding. The library MUST NOT sleep and MUST NOT write HTTP. Prefixing of `key` is the caller's job. Denied requests MUST NOT be treated as backend attempts. Allow MUST NOT return `context.Canceled` or `context.DeadlineExceeded`.

#### Scenario: Closed key admits
- **WHEN** a key is CLOSED with credit above zero
- **AND** Allow is called
- **THEN** Allow returns allowed true
- **AND** retryAfter is zero
- **AND** err is nil

#### Scenario: Caller owns cancellation before Allow
- **WHEN** the caller's request context is already done
- **AND** the caller does not invoke Allow
- **THEN** the gate's credit and state for that key do not change on that request path

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Unit and Yaegi prove Allow
Compiled tests SHALL prove consecutive-failure trip, success credit, idle drop, and deny refreshes TTL, without Traefik. Interpreted tests SHALL run an Allow-then-Report trip with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time. `TestAlloc*` SHALL fail when the warm Allow path (existing key, CLOSED) exceeds the recorded allocs/op and B/op ceilings, and SHALL skip under the race detector.

#### Scenario: Yaegi Allow then Report trips
- **WHEN** the Yaegi GOPATH interp loads non-test `backendbackoff` sources
- **AND** the probe Allows and Reports failures `B` times
- **THEN** the next Allow is denied
- **AND** the interp MUST NOT enable unsafe
