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
`Init(host, pass, database)` SHALL store those three values on the client. `Init` MUST NOT open a TCP connection. Callers SHALL call `Init` once before concurrent use. `InitWithOptions(host, pass, database, Options)` SHALL store those three values and copy `Options` (`DialTimeout`, `IoTimeout`, `IdleTimeout` as durations; `MaxIdleConns` as int). `Init` SHALL delegate to `InitWithOptions` with a zero `Options`. `InitWithOptions` MUST NOT open a TCP connection. Host, password, and database MUST NOT be exported fields. `Options` MUST NOT include `poolSize` or `poolTimeout`. Zero or negative durations, and `MaxIdleConns` of 0 or less, SHALL mean the session defaults (two-second dial, one-second I/O, thirty-second idle, idle list eight). `Options` MUST be a plain struct of those primitives (no functional-option closures).

#### Scenario: Init does not open a socket
- **WHEN** `Init` is called with a host that refuses connections
- **THEN** `Init` returns without error
- **AND** no TCP connection is opened

#### Scenario: InitWithOptions does not open a socket
- **WHEN** `InitWithOptions` is called with a host that refuses connections and a non-zero `IoTimeout`
- **THEN** `InitWithOptions` returns without error
- **AND** no TCP connection is opened

### Requirement: First command dials TCP
The first `Get`, `MGet` (with at least one name), `Set`, or `Del` after `Init` SHALL dial `tcp` to the host stored by `Init`. The session MUST NOT dial a Unix socket and MUST NOT use TLS. Dial timeout SHALL be `DialTimeout`. When `DialTimeout` is 0 or negative, dial timeout SHALL be two seconds.

#### Scenario: Unreachable host
- **WHEN** a command is issued after `Init` with host `127.0.0.1:1`
- **THEN** the command returns an error whose `Error()` text is `redis:unreachable`

#### Scenario: Configured DialTimeout expires under the default
- **WHEN** `InitWithOptions` sets `DialTimeout` to tens of milliseconds
- **AND** a command dials `192.0.2.1`
- **THEN** the command returns `redis:unreachable`
- **AND** that return happens well under two seconds

### Requirement: AUTH and SELECT run once per dial
When `pass` is non-empty, each new dial SHALL send `AUTH` with that password before other commands. When `database` is non-empty, each new dial SHALL send `SELECT` with that database before other commands. Subsequent commands on a reused connection MUST NOT send `AUTH` or `SELECT` again.

#### Scenario: Auth and select once per dial
- **WHEN** the client is Inited with a password and a database
- **AND** several Gets reuse one connection
- **THEN** AUTH and SELECT are sent once for that connection
- **AND** they are not sent again on those Gets

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (eight when `MaxIdleConns` is 0 or less). A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections. An idle connection older than `IdleTimeout` (thirty seconds when `IdleTimeout` is 0 or negative) SHALL not be reused; the next command SHALL dial a new one. A dead pooled connection SHALL be retried once unless the error is a timeout. The idle cap SHALL apply to the idle list after release. It MUST NOT be a live-socket or wait-queue bound.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Configured IdleTimeout opens a new connection
- **WHEN** `InitWithOptions` sets `IdleTimeout` shorter than the default
- **AND** a Get reuses a connection
- **AND** that idle connection is older than that `IdleTimeout`
- **AND** a later Get is issued
- **THEN** the fake observes a second TCP connection

#### Scenario: Configured MaxIdleConns closes extra idle sockets
- **WHEN** `InitWithOptions` sets `MaxIdleConns` to `1`
- **AND** two connections would otherwise sit idle
- **THEN** the idle list length is at most one
- **AND** the extra socket is closed

### Requirement: Close drains the pool and blocks redial
`Close` SHALL close idle pooled connections and mark the client closed. After `Close`, `Get`, `MGet`, `Set`, and `Del` SHALL return `redis:unreachable` and MUST NOT dial. `Close` SHALL be idempotent. In-flight commands MAY finish; their sockets SHALL be closed on release.

#### Scenario: Close drains idle and does not redial
- **WHEN** a client has an idle pooled connection
- **AND** `Close` is called
- **AND** a later Get is issued
- **THEN** that Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). A timeout on a reused connection MUST NOT be retried. Each command SHALL set one deadline of now plus `IoTimeout` covering write and read. When `IoTimeout` is 0 or negative, `IoTimeout` SHALL be one second. The session MUST NOT add separate read and write deadline fields.

#### Scenario: I/O timeout is redis:timeout
- **WHEN** the Redis peer does not complete a reply before the I/O deadline
- **THEN** the command returns `redis:timeout`

#### Scenario: Configured IoTimeout fires instead of the default
- **WHEN** `InitWithOptions` sets `IoTimeout` to tens of milliseconds
- **AND** a fake Redis peer never replies
- **THEN** the command returns `redis:timeout`
- **AND** that return happens well under one second

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

### Requirement: Live I/O timeout is proven on Redis and Dragonfly
Compiled same-package tests SHALL prove a configured I/O deadline against a real Redis and a real Dragonfly. Those tests SHALL table-drive addresses from `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or an address is missing. CI MUST start both engines, set both addresses, and MUST NOT skip those tests with `-short`. Fake-server tests MUST NOT be the only proof. The stall SHALL be Redis `BLPOP` on a unique empty list with a server timeout of ten seconds and an `IoTimeout` of tens of milliseconds. The command SHALL return `redis:timeout` before the server unblocks. The session MUST NOT export a public BLPOP method. The stall MUST NOT use `EVAL`, `CLIENT PAUSE`, or `DEBUG SLEEP`. Traefik, Pester, and the nested probe MUST NOT be the I/O-timeout proof. Probe `Config` MUST remain host-only. If a later proof uses Eval, that Eval MUST be Lua 5.1-safe (no `table.maxn`) and MUST list every touched key in KEYS.

#### Scenario: Configured IoTimeout fires on Redis BLPOP
- **WHEN** CI runs `go test` without `-short` with `SIMPLEREDIS_LIVE_REDIS` set
- **AND** the client is Inited with `IoTimeout` of tens of milliseconds
- **AND** a same-package test sends `BLPOP` on a unique empty key with server timeout `10`
- **THEN** the command returns `redis:timeout`
- **AND** that return happens well before ten seconds

#### Scenario: Configured IoTimeout fires on Dragonfly BLPOP
- **WHEN** CI runs `go test` without `-short` with `SIMPLEREDIS_LIVE_DRAGONFLY` set
- **AND** the client is Inited with `IoTimeout` of tens of milliseconds
- **AND** a same-package test sends `BLPOP` on a unique empty key with server timeout `10`
- **THEN** the command returns `redis:timeout`
- **AND** that return happens well before ten seconds
