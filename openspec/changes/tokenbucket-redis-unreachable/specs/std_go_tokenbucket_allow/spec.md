# Spec Delta

## MODIFIED Requirements

### Requirement: Redis errors do not become deny
When the Redis store is used and Redis is unreachable or times out, Allow SHALL return that error. Callers SHALL match it with `errors.Is` against `simpleredis.ErrUnreachable` or `simpleredis.ErrTimeout`. The display text remains `redis:unreachable` or `redis:timeout`. The returned `Status` SHALL be the zero value (`Allowed` false). The library MUST NOT fail-open, fail-close, or denyOnError inside Allow. Fail-open or fail-closed is the middleware's decision.

#### Scenario: Unreachable Redis
- **WHEN** Allow cannot complete the script because the server is unreachable
- **THEN** Allow returns an error that matches `errors.Is(err, simpleredis.ErrUnreachable)`
- **AND** the returned `Status` is the zero value
- **AND** its `Allowed` is false
- **AND** does not return allowed false as a substitute
