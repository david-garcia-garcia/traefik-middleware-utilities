## ADDED Requirements

### Requirement: Buffered outage is a per-node cap with a nil error
When `sync_rate` is greater than zero and Redis is unreachable or times out, Take and Peek SHALL return the local admit or deny and a nil error. Admit SHALL continue until this instance's sliding estimate exceeds `limit`, then SHALL deny. Peek SHALL use the same estimate without recording a hit. Combined allowed hits across instances MUST NOT be treated as a global cap while Redis is down. Take and Peek MUST NOT return `redis:unreachable` or `redis:timeout` on this path. They MUST NOT GET or INCR on every buffered Take to detect the outage. They MUST NOT return `redis:unreachable` solely because a pending local delta skipped GET. This replaces dest's fail-closed rule that a nil error with a local admit MUST NOT occur.

#### Scenario: Pending buffered delta then kill
- **WHEN** `sync_rate` is greater than zero
- **AND** Take has left a local delta unflushed
- **AND** Redis becomes unreachable
- **AND** Take is called again
- **THEN** Take returns a nil error
- **AND** Take is allowed until this instance's estimate exceeds `limit`
- **AND** the next Take after that is denied
- **AND** Peek on that limiter also returns a nil error

#### Scenario: Successful flush then kill
- **WHEN** `sync_rate` is greater than zero
- **AND** a Take is followed by a successful flush
- **AND** Redis is then killed
- **AND** Take is called
- **THEN** Take returns a nil error
- **AND** allowed follows this instance's remaining room to `limit`

## MODIFIED Requirements

### Requirement: Redis errors propagate
When `sync_rate` is zero and Redis is unreachable or times out, Take and Peek SHALL return that error (`redis:unreachable` or `redis:timeout`). Exact-mode Take and Peek MUST NOT fail-open, fail-close, or health-gate inside the call. Buffered mode (`sync_rate` greater than zero) is not this requirement: a nil error with a local admit or deny is the buffered-outage contract.

#### Scenario: Unreachable Redis
- **WHEN** `sync_rate` is zero
- **AND** Take cannot complete the Redis increment or previous-window read because the server is unreachable
- **THEN** Take returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

#### Scenario: Unreachable Redis on Peek
- **WHEN** `sync_rate` is zero
- **AND** Peek cannot complete the Redis read because the server is unreachable
- **THEN** Peek returns `redis:unreachable`
- **AND** MUST NOT admit or deny as a silent fallback

### Requirement: Unit and interpreter tests prove Take without Traefik
Compiled unit tests SHALL prove encoder and window math against an in-process fake TCP Redis (no Docker), including Peek-does-not-increment, Peek-agrees-with-Take, and sliding cooldown via the formula. That fake SHALL be able to close its listener and every live socket on demand so a pending-delta outage can be proven. Interpreter tests that import Yaegi SHALL run live Take and Peek scenarios against GOPATH copies of non-test `windowcounter` and `simpleredis` sources, stdlib symbols only, `useunsafe` false. Those interpreter tests MUST NOT start Traefik. The compiled test owns process start and skip.

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
- **AND** Take is called again
- **THEN** Take returns a nil error
- **AND** further Takes on that instance are allowed until `limit`, then denied
