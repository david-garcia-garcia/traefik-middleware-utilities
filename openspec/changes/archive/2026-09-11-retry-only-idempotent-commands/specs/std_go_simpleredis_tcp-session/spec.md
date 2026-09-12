## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most eight. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one. A dead pooled connection SHALL be retried once when the command is GET, MGET, SET, DEL, EXPIRE, or EXPIREAT, unless the error is a timeout. INCR, INCRBY, and EVAL MUST NOT be retried after a dead reused socket; they SHALL return `redis:unreachable` and MUST NOT send the command a second time. A timeout on a reused connection MUST NOT be retried for any verb.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Lost-reply Incr is not retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer applies INCR then closes before writing the reply
- **AND** Incr is called once
- **THEN** Incr returns `redis:unreachable`
- **AND** the stored value for that key is `1`
- **AND** the peer observes only one INCR

#### Scenario: Lost-reply Eval is not retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer applies EVAL then closes before writing the reply
- **AND** Eval is called once with a Lua 5.1-safe script that lists its key in KEYS
- **THEN** Eval returns `redis:unreachable`
- **AND** the stored script result is the value after one apply
- **AND** the peer observes only one EVAL

#### Scenario: Lost-reply Get is retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer closes before writing the GET reply
- **AND** Get is called
- **THEN** Get returns the stored bytes
- **AND** the peer observes a second GET on a new connection

## ADDED Requirements

### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, in addition to the happy-path verbs, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL forward the command, wait until the engine has applied it, and close without writing that reply when the verb is INCR, INCRBY, or EVAL. Other verbs SHALL pass through. The plugin SHALL set `X-SimpleRedis-DropIncr` to `redis:unreachable`, `X-SimpleRedis-DropIncrStored` to the stored value `1` read from the engine (not the drop-relay), `X-SimpleRedis-DropEval` to `redis:unreachable`, and `X-SimpleRedis-DropEvalStored` to the stored script result. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** a request is made on `/redis`
- **THEN** `X-SimpleRedis-DropIncr` is `redis:unreachable`
- **AND** `X-SimpleRedis-DropIncrStored` is `1`
- **AND** `X-SimpleRedis-DropEval` is `redis:unreachable`
- **AND** `X-SimpleRedis-DropEvalStored` is the stored script result after one apply
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** a request is made on `/dragonfly`
- **THEN** `X-SimpleRedis-DropIncr` is `redis:unreachable`
- **AND** `X-SimpleRedis-DropIncrStored` is `1`
- **AND** `X-SimpleRedis-DropEval` is `redis:unreachable`
- **AND** `X-SimpleRedis-DropEvalStored` is the stored script result after one apply
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`
