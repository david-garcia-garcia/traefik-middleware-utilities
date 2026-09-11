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

### Requirement: Handshake AUTH or SELECT failure closes and is not pooled
When AUTH on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool. AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. When SELECT on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool, and SHALL return that error text. `ERR DB index is out of range` MUST NOT map to `redis:noauth`. AUTH SHALL run before SELECT when both password and database are non-empty. A handshake failure SHALL surface one error to the caller and MUST NOT open a second TCP connection for that command. In-process handshake-failure tests SHALL use a fake whose AUTH and SELECT replies are configurable (default success so existing success tests stay). Live Redis and Dragonfly tests SHALL prove the cases each dest engine supports and SHALL skip when those engines are unset.

#### Scenario: Fake AUTH rejected maps to redis:noauth and is not pooled
- **WHEN** the client is Inited with a non-empty password and an empty database
- **AND** the fake replies to AUTH with an AUTH-class prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, or `ERR Client sent AUTH`)
- **AND** a command is issued
- **THEN** the command returns `redis:noauth`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake SELECT rejected after AUTH is not pooled
- **WHEN** the client is Inited with a password and database `99`
- **AND** the fake replies `+OK` to AUTH and `-ERR DB index is out of range` to SELECT
- **AND** a command is issued
- **THEN** AUTH is sent before SELECT
- **AND** the command returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Live SELECT 99 on Redis and Dragonfly
- **WHEN** dest Redis and Dragonfly are reachable without a password
- **AND** the client is Inited with database `99`
- **AND** a command is issued
- **THEN** each engine returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **WHEN** those engines are unset
- **THEN** the live SELECT tests skip

#### Scenario: Live wrong password on Redis and Dragonfly with requirepass
- **WHEN** dest Redis and Dragonfly are reachable with requirepass set
- **AND** the client is Inited with a wrong password
- **AND** a command is issued
- **THEN** each engine returns `redis:noauth`
- **AND** the idle pool is empty
- **WHEN** those passworded engines are unset
- **THEN** the live wrong-password tests skip
