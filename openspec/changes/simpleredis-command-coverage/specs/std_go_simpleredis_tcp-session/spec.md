## MODIFIED Requirements

### Requirement: Peer-closed idle socket is retried
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry on a new dial under the go-redis-shaped `MaxRetries` policy. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against both Redis and Dragonfly, then the next command SHALL succeed on a new dial. Those live tests MUST skip under `-short` or when both SimpleRedis live addresses are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. The nested Traefik plugin SHALL keep `simpleredis.New` in Traefik `New`. A recover request (`/redis/recover` and `/dragonfly/recover`) SHALL run Set and Get only, SHALL set `X-SimpleRedis-Recover: ok` when those succeed after recovery, and MUST NOT Eval. Exact `/redis` and `/dragonfly` SHALL remain health Set+Get. Verb cases live on `/redis/<case>` and `/dragonfly/<case>`. Eval on `/eval` SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

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
- **AND** a later request is made on `/redis/recover`
- **THEN** the response status is 200
- **AND** the response includes `X-SimpleRedis-Recover: ok`
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Traefik Dragonfly recover after kill
- **WHEN** a request has already succeeded on `/dragonfly`
- **AND** the probe's pooled Dragonfly connection is killed with `CLIENT KILL` by `ADDR` or `ID`
- **AND** a later request is made on `/dragonfly/recover`
- **THEN** the response status is 200
- **AND** the response includes `X-SimpleRedis-Recover: ok`
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Default verb headers stay
- **WHEN** a request is made on `/redis/get`
- **THEN** the response includes `X-SimpleRedis-Value`
- **AND** GET `/redis/eval` lists its key in `KEYS` and is Lua 5.1-safe

### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, on `/redis/drop` and `/dragonfly/drop`, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL drop INCR, INCRBY, or EVAL only after that TCP session has already forwarded at least one command (the probe warms with GET). A retry on a new session whose first command is INCR or EVAL SHALL pass the reply through. Other verbs SHALL pass through. The plugin SHALL set `X-SimpleRedis-DropIncr` to the integer after two applies (`2`), `X-SimpleRedis-DropIncrStored` to the stored value `2` read from the engine (not the drop-relay), `X-SimpleRedis-DropEval` to the integer after two applies of the Kong script (`6` when ARGV INCRBY is `3`), and `X-SimpleRedis-DropEvalStored` to that stored script result. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** a request is made on `/redis/drop`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** a request is made on `/dragonfly/drop`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`
