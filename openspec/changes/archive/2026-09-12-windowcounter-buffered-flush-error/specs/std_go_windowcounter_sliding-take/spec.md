## MODIFIED Requirements

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
- **AND** the clock advances one `sync_rate` (or a flush is known to have failed)
- **AND** Take is called again
- **THEN** Take returns a non-nil Redis error
