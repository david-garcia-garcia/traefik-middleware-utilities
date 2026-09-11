## Purpose

Sliding-window HTTP rate-limit primitive: admit or deny a hit on an opaque key against a limit and window, using Redis integer counters for the current and previous windows. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Take admits until the sliding estimate exceeds the limit
`Take(key, limit, window)` SHALL increment the current-window counter, then return `allowed` true when the sliding estimate is less than or equal to `limit`, and false otherwise. `Allow` SHALL be the same operation. The estimate SHALL be `current + previous × (1 − elapsed/window)` using the caller's opaque `key`. Denied hits SHALL still occupy the window. Prefixing of `key` is the caller's job.

#### Scenario: N Takes then deny
- **WHEN** the limit is N and the window has no prior hits
- **AND** Take is called N times for the same key
- **THEN** each of those Takes returns allowed true
- **WHEN** Take is called once more for that key in the same window
- **THEN** that Take returns allowed false

#### Scenario: Caller owns the key
- **WHEN** Take is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Sliding estimate uses current and previous windows
The current window start SHALL be `floor(unixSeconds / windowSeconds) × windowSeconds`. Redis keys SHALL be `{opaqueKey}:{windowStart}` and `{opaqueKey}:{previousWindowStart}`. A missing previous window SHALL count as zero. Window length SHALL be a whole number of seconds. Sub-second windows MUST NOT be supported.

#### Scenario: Dump at the window boundary does not double the limit
- **WHEN** a key has used its full limit near the end of a window
- **AND** Take is called at the start of the next window
- **THEN** the estimate still includes the previous window's hits
- **AND** the new window MUST NOT admit a second full limit the way a fixed window would

#### Scenario: Usage is the sliding estimate
- **WHEN** Take returns
- **THEN** the usage value is the sliding estimate after this Take as a float
- **AND** it is not remaining quota and not an integer ceiling of the estimate

### Requirement: Redis errors propagate
When Redis is unreachable or times out, Take SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or health-gate inside Take.

#### Scenario: Unreachable Redis
- **WHEN** Take cannot complete the Redis increment or previous-window read because the server is unreachable
- **THEN** Take returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

### Requirement: Unit and interpreter tests prove Take without Traefik
Compiled unit tests SHALL prove encoder and window math against an in-process fake TCP Redis (no Docker). Interpreter tests that import Yaegi SHALL run the same live Take scenarios against GOPATH copies of non-test `ratelimit` and `simpleredis` sources, stdlib symbols only, `useunsafe` false. Those interpreter tests MUST NOT start Traefik. The compiled test owns process start and skip.

#### Scenario: Yaegi Take against a fake
- **WHEN** interpreted code constructs a limiter on a compiled fake Redis
- **AND** it calls Take until the limit
- **THEN** the next Take is denied
- **AND** Traefik is not started
