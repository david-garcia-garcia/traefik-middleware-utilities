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
The session SHALL keep unused TCP connections in an idle pool of at most eight. Live sockets (idle plus checked out) SHALL not exceed eight. When idle is empty and live sockets are already eight, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections, including when more than eight callers overlap. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one if live sockets are under eight. A dead pooled connection SHALL be retried once unless the error is a timeout. `release` MUST NOT close a reusable socket solely because the idle list is full while live sockets are under eight.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Overlapping callers above eight do not dial past the live cap
- **WHEN** more than eight concurrent commands overlap against a live fake Redis
- **THEN** the fake observes at most eight TCP connections
- **AND** waiters reuse a released socket rather than opening a ninth

### Requirement: Full pool wait returns redis:unreachable
When every live socket is checked out, a further command SHALL wait for a slot. If no slot frees before the pool wait (200 milliseconds) elapses, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library (no extra timer goroutine leak: stop the timer when a slot arrives).

#### Scenario: Pool wait times out
- **WHEN** all eight live sockets are busy
- **AND** another command is issued
- **AND** no socket becomes free within 200 milliseconds
- **THEN** that command returns `redis:unreachable`
- **AND** the fake or server observes no additional TCP connection for that command

### Requirement: Live cap is proven on Redis and Dragonfly
The session SHALL keep at most eight live TCP connections (idle plus in use) against a real Redis and a real Dragonfly. Fake-server tests MUST NOT be the only proof. Traefik local-plugin Pester on `/redis` and `/dragonfly` SHALL overlap requests long enough to contend for sockets, observe at most eight clients on that backend, and observe `redis:unreachable` when a waiter exceeds the pool wait. Compiled tests gated on live addresses SHALL prove the same two facts on both engines. Any Lua used to hold a socket MUST be Lua 5.1-safe (no `table.maxn`) and MUST list touched keys in KEYS (zero keys when none are touched). Compose project `reclaim-e2e`, routes `/a` `/b`, and existing verb headers MUST keep their semantics.

#### Scenario: Concurrent holds stay within eight on Redis
- **WHEN** overlapping requests through the Traefik plugin hold sockets against compose Redis
- **THEN** Redis has at most eight established TCP clients from that plugin
- **AND** reclaim routes `/a` and `/b` still succeed

#### Scenario: Concurrent holds stay within eight on Dragonfly
- **WHEN** overlapping requests through the Traefik plugin hold sockets against compose Dragonfly
- **THEN** Dragonfly has at most eight established TCP clients from that plugin

#### Scenario: Extra waiter is redis:unreachable on both engines
- **WHEN** eight sockets are held against Redis
- **AND** another request cannot obtain a socket before the pool wait
- **THEN** that request fails with `redis:unreachable`
- **WHEN** the same overlap is run against Dragonfly
- **THEN** that request fails with `redis:unreachable`

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
