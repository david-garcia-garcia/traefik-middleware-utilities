## Purpose

Defines the stdlib TCP session a SimpleRedis client holds: `Init` records host, password, and database without dialing; the first command dials; AUTH and SELECT run once per dial; idle connections are pooled; `Close` drains the pool and blocks further dials. Callers import `simpleredis` from this module. The session loads under Traefik Yaegi from a nested fake plugin that Inits in `New`.

## Requirements

### Requirement: Session source depends only on the Go standard library
The SimpleRedis session source SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, e2e, or vendor packages. It MUST NOT import a Redis client module (`go-redis`, miniredis, or similar). It MUST NOT use `unsafe`, cgo, or type parameters. Retry jitter SHALL use `math/rand` (`Int63n`), which is standard library; it MUST NOT use `math/rand/v2`.

#### Scenario: Stdlib-only imports
- **WHEN** the SimpleRedis session source is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Package path is simpleredis
The session SHALL live in package `simpleredis` under folder `simpleredis/`. The import last segment MUST be `simpleredis`. The package clause MUST NOT be `redis`.

#### Scenario: Import last segment is simpleredis
- **WHEN** a caller imports the client from this module
- **THEN** the import path ends in `/simpleredis`

### Requirement: Init records settings and does not dial
`Init(host, pass, database)` SHALL store those three values on the client. `Init` MUST NOT open a TCP connection. Callers SHALL call `Init` once before concurrent use.

#### Scenario: Init does not open a socket
- **WHEN** `Init` is called with a host that refuses connections
- **THEN** `Init` returns without error
- **AND** no TCP connection is opened

### Requirement: First command dials TCP
The first `Get`, `MGet` (with at least one name), `Set`, or `Del` after `Init` SHALL dial `tcp` to the host stored by `Init`. The session MUST NOT dial a Unix socket and MUST NOT use TLS. Dial timeout SHALL be two seconds.

#### Scenario: Unreachable host
- **WHEN** a command is issued after `Init` with host `127.0.0.1:1`
- **THEN** the command returns an error whose `Error()` text is `redis:unreachable`

### Requirement: AUTH and SELECT run once per dial
When `pass` is non-empty, each new dial SHALL send `AUTH` with that password before other commands. When `database` is non-empty, each new dial SHALL send `SELECT` with that database before other commands. Subsequent commands on a reused connection MUST NOT send `AUTH` or `SELECT` again.

#### Scenario: Auth and select once per dial
- **WHEN** the client is Inited with a password and a database
- **AND** several Gets reuse one connection
- **THEN** AUTH and SELECT are sent once for that connection
- **AND** they are not sent again on those Gets

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most eight. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry.

Every command (GET, MGET, SET, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL) SHALL use go-redis-shaped command retry. Exported fields `MaxRetries`, `MinRetryBackoff`, and `MaxRetryBackoff` follow go-redis Options sentinels and MUST NOT be Init arguments: `0` means the default (3 extra retries, 8ms, 512ms); `-1` means off (no extra retries, no backoff sleep). The zero-value client therefore retries like go-redis. The retry loop SHALL be `for attempt := 0; attempt <= maxRetries; attempt++` (default 3 means at most four sends). Backoff SHALL sleep only between retries (`attempt > 0`), using go-redis `RetryBackoff` (`math/rand` `Int63n` jitter). Init stays `Init(host, pass, database)`.

A command SHALL retry when the error is `redis:unreachable` (EOF, unexpected EOF, dial failure, and other IO as this client maps them), including a fresh dial, unless the client is `Close`d (`redis:unreachable` from a closed client MUST NOT spin `MaxRetries`). A command SHALL also retry Redis error replies whose text is `ERR max number of clients reached` or has prefix `LOADING `, `READONLY `, `MASTERDOWN `, `CLUSTERDOWN `, or `TRYAGAIN ` (space after the word). A command MUST NOT retry `redis:timeout` (documented deviation from go-redis: `ioTimeout` is one second; retrying would stall a Traefik request for several seconds), `redis:miss`, `redis:noauth`, `redis:issue?`, or other Redis `-ERR` replies. This client has no waiting pool and MUST NOT invent a pool timeout.

INCR, INCRBY, and EVAL MAY double-apply when a reply is lost and the command is retried. That is accepted. A timeout on a reused connection MUST NOT be retried for any verb.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Lost-reply Incr is retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer applies INCR then closes before writing the reply, once
- **AND** Incr is called once with default MaxRetries
- **THEN** Incr succeeds
- **AND** the stored value for that key is `2`
- **AND** the peer observes two INCR commands
- **AND** the peer observes a second TCP connection

#### Scenario: Lost-reply Incr with MaxRetries off
- **WHEN** a client has an idle pooled connection
- **AND** `MaxRetries` is `-1`
- **AND** the peer applies INCR then closes before writing the reply, once
- **AND** Incr is called once
- **THEN** Incr returns `redis:unreachable`
- **AND** the stored value for that key is `1`
- **AND** the peer observes only one INCR

#### Scenario: Lost-reply Eval is retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer applies EVAL then closes before writing the reply, once
- **AND** Eval is called once with a Lua 5.1-safe script that lists its key in KEYS
- **THEN** Eval succeeds
- **AND** the stored script result is the value after two applies
- **AND** the peer observes two EVAL commands
- **AND** the peer observes a second TCP connection

#### Scenario: Lost-reply Get is retried
- **WHEN** a client has an idle pooled connection
- **AND** the peer closes before writing the GET reply
- **AND** Get is called
- **THEN** Get returns the stored bytes
- **AND** the peer observes a second GET on a new connection

### Requirement: Close drains the pool and blocks redial
`Close` SHALL close idle pooled connections and mark the client closed. After `Close`, `Get`, `MGet`, `Set`, and `Del` SHALL return `redis:unreachable` and MUST NOT dial. `Close` SHALL be idempotent. In-flight commands MAY finish; their sockets SHALL be closed on release.

#### Scenario: Close drains idle and does not redial
- **WHEN** a client has an idle pooled connection
- **AND** `Close` is called
- **AND** a later Get is issued
- **THEN** that Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). `redis:timeout` MUST NOT be retried (documented deviation from go-redis; `ioTimeout` is one second). A timeout on a reused connection MUST NOT open a second connection.

#### Scenario: I/O timeout is redis:timeout
- **WHEN** the Redis peer does not complete a reply before the I/O deadline
- **THEN** the command returns `redis:timeout`

### Requirement: Library Init loads under Traefik Yaegi
A Traefik local plugin SHALL import this module’s `simpleredis` package. `New` SHALL call `Init` only (no command, so Traefik still starts if Redis is late). Traefik SHALL start. A request through that plugin SHALL succeed. `useunsafe` MUST be false. That plugin MUST be a nested module, not this repo’s root `plugin.go`. The existing reclaim e2e compose project, Traefik container, ports 8000/8080, and routes `/a` `/b` MUST keep their reclaim semantics.

#### Scenario: Fake plugin starts without dialing
- **WHEN** Traefik v3.7.11 loads a local plugin whose `New` calls `SimpleRedis.Init`
- **AND** Redis is not yet accepting connections
- **THEN** Traefik’s API is reachable

#### Scenario: Request through the plugin succeeds
- **WHEN** Traefik has loaded that plugin
- **AND** compose Redis is reachable at `redis:6379` with no password
- **AND** a request is made on the plugin’s whoami route
- **THEN** the request succeeds
- **AND** reclaim routes `/a` and `/b` still succeed

#### Scenario: Reclaim whoami are not stopped for Redis proof
- **WHEN** the Redis Pester Describe runs
- **THEN** it does not stop `whoami-a` or `whoami-b`

### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, in addition to the happy-path verbs, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL drop INCR, INCRBY, or EVAL only after that TCP session has already forwarded at least one command (the probe warms with GET). A retry on a new session whose first command is INCR or EVAL SHALL pass the reply through. Other verbs SHALL pass through. The plugin SHALL set `X-SimpleRedis-DropIncr` to the integer after two applies (`2`), `X-SimpleRedis-DropIncrStored` to the stored value `2` read from the engine (not the drop-relay), `X-SimpleRedis-DropEval` to the integer after two applies of the Kong script (`6` when ARGV INCRBY is `3`), and `X-SimpleRedis-DropEvalStored` to that stored script result. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** a request is made on `/redis`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** a request is made on `/dragonfly`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`
