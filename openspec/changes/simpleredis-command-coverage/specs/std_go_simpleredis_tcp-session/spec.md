## MODIFIED Requirements

### Requirement: Peer-closed idle socket is retried
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry on a new dial under the go-redis-shaped `MaxRetries` policy. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against both Redis and Dragonfly, then the next command SHALL succeed on a new dial. Those live tests MUST skip under `-short` or when both SimpleRedis live addresses are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. The nested Traefik plugin SHALL keep `simpleredis.New` in Traefik `New`. Pester recover SHALL be health warmup, `CLIENT KILL`, then Set and Get only, and MUST NOT Eval. Exact `/redis` and `/dragonfly` SHALL remain health Set+Get. Verb paths live on `/redis/<verb>` and `/dragonfly/<verb>`. Eval on `/eval` SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Peer-closed idle is retried
- **WHEN** a compiled fake Redis accepts one connection, answers the first Get, and closes that accepted socket without reading further
- **AND** a second Get is issued while the pooled socket is still younger than thirty seconds
- **THEN** that Get succeeds on a new dial
- **AND** the fake observed two accepts
- **AND** the dead connection is not in the idle pool

#### Scenario: Retry cannot obtain a connection
- **WHEN** a compiled fake Redis accepts one connection, answers the first Get, closes that accepted socket, and then the listener is closed
- **AND** a second Get is issued
- **THEN** that Get returns `redis:unreachable`

#### Scenario: Client-side close stays a distinct proof
- **WHEN** a pooled idle socket is closed from the client
- **THEN** the next Get is still retried
- **AND** that test MUST NOT close the accepted socket from the server

#### Scenario: Live Redis recovers after CLIENT KILL
- **WHEN** a SimpleRedis client has an idle pooled connection to live Redis
- **AND** a sidecar issues `CLIENT KILL` by `ADDR` or `ID` of that pooled socket
- **AND** the next Get is issued before thirty seconds of client idle
- **THEN** that Get succeeds
- **AND** the killed socket is not reused

#### Scenario: Live Dragonfly recovers after CLIENT KILL
- **WHEN** a SimpleRedis client has an idle pooled connection to live Dragonfly
- **AND** a sidecar issues `CLIENT KILL` by `ADDR` or `ID` of that pooled socket
- **AND** the next Get is issued before thirty seconds of client idle
- **THEN** that Get succeeds
- **AND** the killed socket is not reused

#### Scenario: Traefik Redis recover after kill
- **WHEN** a request has already succeeded on `/redis`
- **AND** the probe's pooled Redis connection is killed with `CLIENT KILL` by `ADDR` or `ID`
- **AND** a later Set then Get is made on `/redis`
- **THEN** the Get status is 200
- **AND** the Get body is the value Set wrote
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Traefik Dragonfly recover after kill
- **WHEN** a request has already succeeded on `/dragonfly`
- **AND** the probe's pooled Dragonfly connection is killed with `CLIENT KILL` by `ADDR` or `ID`
- **AND** a later Set then Get is made on `/dragonfly`
- **THEN** the Get status is 200
- **AND** the Get body is the value Set wrote
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Default verb paths stay
- **WHEN** Pester GETs `/redis/get` after Set
- **THEN** the response body is the value Set wrote
- **AND** POST `/redis/eval` lists its key in `KEYS` and is Lua 5.1-safe

### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, with query `drop=1` on `/redis` and `/dragonfly` verb paths, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL drop INCR, INCRBY, or EVAL only after that TCP session has already forwarded at least one command (Pester warms with GET). A retry on a new session whose first command is INCR or EVAL SHALL pass the reply through. Other verbs SHALL pass through. After drop Incr the body SHALL be the integer after two applies (`2`) and a Get without `drop=1` SHALL return stored `2`. After drop Eval of the Kong script (ARGV INCRBY `3`) the body SHALL be `6` and a Get without `drop=1` SHALL return stored `6`. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** Pester Incrs and Evals `/redis` with `drop=1` after a Get warmup
- **THEN** the Incr body is `2`
- **AND** a Get without `drop=1` of that key is `2`
- **AND** the Eval body is `6`
- **AND** a Get without `drop=1` of that Eval key is `6`
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** Pester Incrs and Evals `/dragonfly` with `drop=1` after a Get warmup
- **THEN** the Incr body is `2`
- **AND** a Get without `drop=1` of that key is `2`
- **AND** the Eval body is `6`
- **AND** a Get without `drop=1` of that Eval key is `6`
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`
