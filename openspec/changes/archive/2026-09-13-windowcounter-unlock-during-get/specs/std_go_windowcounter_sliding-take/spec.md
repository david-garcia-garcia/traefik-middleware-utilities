## ADDED Requirements

### Requirement: Buffered Take does not wait on another key's Redis GET
When `sync_rate` is greater than zero, a Take on one opaque key MUST NOT wait for an in-flight Redis GET that belongs to a different opaque key on the same limiter. Redis GET and EVAL for the local window buffer MUST NOT run while the limiter mutex that serializes that buffer is held. Exact mode (`sync_rate` zero) is unchanged.

#### Scenario: Fast Take during a delayed GET on another key
- **WHEN** `sync_rate` is greater than zero
- **AND** a Take on opaque key `slow` is blocked in Redis GET
- **AND** a Take on a different opaque key `fast` is issued on the same limiter
- **THEN** the Take on `fast` returns before that GET on `slow` finishes
- **AND** the wait is well under the GET delay

## MODIFIED Requirements

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
