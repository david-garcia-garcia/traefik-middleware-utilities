## ADDED Requirements

### Requirement: Peek observes without increment
`Peek(key, limit, window)` SHALL return `allowed` and the sliding estimate using the same arguments, formula, and Redis keys as Take, and MUST NOT increment the current-window counter. `Allow` SHALL remain the same operation as Take and MUST NOT become Peek.

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

## MODIFIED Requirements

### Requirement: Redis errors propagate
When Redis is unreachable or times out, Take and Peek SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or health-gate inside Take or Peek.

#### Scenario: Unreachable Redis
- **WHEN** Take cannot complete the Redis increment or previous-window read because the server is unreachable
- **THEN** Take returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

#### Scenario: Unreachable Redis on Peek
- **WHEN** Peek cannot complete the Redis read because the server is unreachable
- **THEN** Peek returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

### Requirement: Unit and interpreter tests prove Take without Traefik
Compiled unit tests SHALL prove encoder and window math against an in-process fake TCP Redis (no Docker), including Peek-does-not-increment, Peek-agrees-with-Take, and sliding cooldown via the formula. Interpreter tests that import Yaegi SHALL run live Take and Peek scenarios against GOPATH copies of non-test `windowcounter` and `simpleredis` sources, stdlib symbols only, `useunsafe` false. Those interpreter tests MUST NOT start Traefik. The compiled test owns process start and skip.

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
