## Purpose

Defines the stdlib TCP session a SimpleRedis client holds: `Init` records host, password, and database without dialing; the first command dials; AUTH and SELECT run once per dial; idle connections are pooled; `Close` drains the pool and blocks further dials. Callers import `simpleredis` from this module. The session loads under Traefik Yaegi from a nested fake plugin that Inits in `New`.

## Requirements

### Requirement: Session source depends only on the Go standard library
The SimpleRedis session source SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, e2e, or vendor packages. It MUST NOT import a Redis client module (`go-redis`, miniredis, or similar). It MUST NOT use `unsafe`, cgo, or type parameters.

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
The session SHALL keep unused TCP connections in an idle pool of at most eight. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one. A dead pooled connection SHALL be retried once unless the error is a timeout.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

### Requirement: Close drains the pool and blocks redial
`Close` SHALL close idle pooled connections and mark the client closed. After `Close`, `Get`, `MGet`, `Set`, and `Del` SHALL return `redis:unreachable` and MUST NOT dial. `Close` SHALL be idempotent. In-flight commands MAY finish; their sockets SHALL be closed on release.

#### Scenario: Close drains idle and does not redial
- **WHEN** a client has an idle pooled connection
- **AND** `Close` is called
- **AND** a later Get is issued
- **THEN** that Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). A timeout on a reused connection MUST NOT be retried.

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

### Requirement: Peer-closed idle socket is retried once
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry once on a new dial. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against both Redis and Dragonfly, then the next command SHALL succeed on a new dial. The nested Traefik plugin SHALL keep `Init` in `New`. A recover request (`recover=1`) SHALL run Set and Get only, SHALL set `X-SimpleRedis-Recover: ok` when those succeed after recovery, and MUST NOT Eval. Default `/redis` and `/dragonfly` verb headers MUST stay. Existing Eval on the default path SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Peer-closed idle is retried once
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
- **THEN** the next Get is still retried once
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
- **AND** a later request is made on `/redis?recover=1`
- **THEN** the response status is 200
- **AND** the response includes `X-SimpleRedis-Recover: ok`
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Traefik Dragonfly recover after kill
- **WHEN** a request has already succeeded on `/dragonfly`
- **AND** the probe's pooled Dragonfly connection is killed with `CLIENT KILL` by `ADDR` or `ID`
- **AND** a later request is made on `/dragonfly?recover=1`
- **THEN** the response status is 200
- **AND** the response includes `X-SimpleRedis-Recover: ok`
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Default verb headers stay
- **WHEN** a request is made on `/redis` or `/dragonfly` without `recover=1`
- **THEN** the response still includes the existing verb headers
- **AND** that request's Eval lists its key in `KEYS` and is Lua 5.1-safe
