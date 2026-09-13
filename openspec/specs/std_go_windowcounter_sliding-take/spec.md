## Purpose

Sliding-window hit counter: admit or deny a hit on an opaque key against a limit and window, using Redis integer counters for the current and previous windows. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Take admits until the sliding estimate exceeds the limit
`Take(ctx, key, limit, window)` SHALL increment the current-window counter, then return `allowed` true when the sliding estimate is less than or equal to `limit`, and false otherwise. `Allow` SHALL be the same operation. The first argument SHALL be a `context.Context`. A caller with no deadline SHALL pass `context.Background()`. The estimate SHALL be `current + previous × (1 − elapsed/window)` using the caller's opaque `key`. Denied hits SHALL still occupy the window. Prefixing of `key` is the caller's job.

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

### Requirement: Peek observes without increment
`Peek(ctx, key, limit, window)` SHALL return `allowed` and the sliding estimate using the same arguments, formula, and Redis keys as Take, and MUST NOT increment the current-window counter. `Allow` SHALL remain the same operation as Take and MUST NOT become Peek. The first argument SHALL be a `context.Context`.

#### Scenario: N Peeks then Take sees count 1
- **WHEN** the window has no prior hits
- **AND** Peek is called N times for the same key, limit, and window
- **THEN** each Peek returns allowed true and the estimate does not grow from those Peeks
- **WHEN** Take is then called once for that key
- **THEN** the sliding estimate after that Take equals one hit, not N plus one

### Requirement: Peek agrees with Take before the increment
For the same clock, key, limit, and window, Peek's `allowed` and estimated SHALL match the values Take would return for that same state before Take increments.

#### Scenario: Peek then Take at a frozen clock
- **WHEN** the clock is held fixed
- **AND** Peek is called on a key
- **AND** Take is then called on that same key, limit, and window
- **THEN** Take's allowed matches Peek's allowed
- **AND** Take's estimated equals Peek's estimated plus the one new hit's contribution

### Requirement: Peek follows the sliding window after Takes stop
After enough Takes to deny, further Peeks with no Takes SHALL stay denied while the weighted estimate remains above the limit, then SHALL become allowed when the clock advances enough that the formula (`current + previous × (1 − elapsed/window)`) drops to at most the limit. The cooldown MUST be that formula at the caller's clock, not a wait of two window lengths.

#### Scenario: Denied Peek becomes allowed as the window slides
- **WHEN** Takes have filled the window so the estimate exceeds the limit
- **AND** Peek is called with no further Takes at that same clock
- **THEN** Peek returns allowed false
- **WHEN** the clock advances within the sliding formula until the estimate is at most the limit
- **AND** Peek is called again with no Takes in between
- **THEN** Peek returns allowed true

### Requirement: Redis errors propagate
When Redis is unreachable or times out, Take and Peek SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or health-gate inside Take or Peek. This includes buffered mode (`sync_rate` greater than zero) while a local delta is still pending: a nil error with a local admit is a silent fallback and MUST NOT occur once a flush has failed or one `sync_rate` has passed without a successful Redis contact.

#### Scenario: Unreachable Redis
- **WHEN** Take cannot complete the Redis increment or previous-window read because the server is unreachable
- **THEN** Take returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

#### Scenario: Unreachable Redis on Peek
- **WHEN** Peek cannot complete the Redis read because the server is unreachable
- **THEN** Peek returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

#### Scenario: Unreachable Redis with a pending buffered delta
- **WHEN** `sync_rate` is greater than zero
- **AND** Take has left a local delta unflushed
- **AND** Redis becomes unreachable
- **AND** Take or Peek is called after a failed flush, or after one `sync_rate` without a successful Redis contact
- **THEN** that call returns `redis:unreachable` or `redis:timeout`
- **AND** MUST NOT return a nil error

### Requirement: Buffered Take does not wait on another key's Redis GET
When `sync_rate` is greater than zero, a Take on one opaque key MUST NOT wait for an in-flight Redis GET that belongs to a different opaque key on the same limiter. Redis GET and EVAL for the local window buffer MUST NOT run while the limiter mutex that serializes that buffer is held. Exact mode (`sync_rate` zero) is unchanged.

#### Scenario: Fast Take during a delayed GET on another key
- **WHEN** `sync_rate` is greater than zero
- **AND** a Take on opaque key `slow` is blocked in Redis GET
- **AND** a Take on a different opaque key `fast` is issued on the same limiter
- **THEN** the Take on `fast` returns before that GET on `slow` finishes
- **AND** the wait is well under the GET delay

### Requirement: Unit and interpreter tests prove Take without Traefik
Compiled unit tests SHALL prove encoder and window math against an in-process fake TCP Redis (no Docker), including Peek-does-not-increment, Peek-agrees-with-Take, sliding cooldown via the formula, and that a buffered Take on one opaque key does not wait for a delayed GET on a different opaque key. That fake SHALL be able to close its listener and every live socket on demand so a pending-delta outage can be proven. That fake SHALL be able to hold a GET whose Redis key matches a prefix until release or a hold duration, without holding the fake's own mutex during that hold. Interpreter tests that import Yaegi SHALL run live Take and Peek scenarios against GOPATH copies of non-test `windowcounter` and `simpleredis` sources, stdlib symbols only, `useunsafe` false. Those interpreter tests MUST NOT start Traefik. The compiled test owns process start and skip.

#### Scenario: Yaegi Take against a fake
- **WHEN** interpreted code constructs a limiter on a compiled fake Redis
- **AND** it calls Take until the limit
- **THEN** the next Take is denied
- **AND** Traefik is not started

#### Scenario: Yaegi Peek and Take against a fake
- **WHEN** interpreted code constructs a limiter on a compiled fake Redis
- **AND** it calls Peek then Take
- **THEN** Peek does not increment
- **AND** Take records one hit
- **AND** Traefik is not started

#### Scenario: Pending buffered delta then kill
- **WHEN** `sync_rate` is long enough that no tick fires
- **AND** one Take succeeds while Redis is up, leaving a local delta
- **AND** the fake closes its listener and live sockets
- **AND** the clock advances one `sync_rate` (or a flush is known to have failed)
- **AND** Take is called again
- **THEN** Take returns a non-nil Redis error

#### Scenario: Fast Take while GET on another key is held
- **WHEN** `sync_rate` is greater than zero
- **AND** the fake holds GET for Redis keys with prefix `slow:`
- **AND** a Take on opaque key `slow` has entered that hold
- **AND** a Take on opaque key `fast` is issued on the same limiter
- **THEN** the Take on `fast` returns well under the GET hold duration
- **AND** it MUST NOT wait for the held GET to finish
