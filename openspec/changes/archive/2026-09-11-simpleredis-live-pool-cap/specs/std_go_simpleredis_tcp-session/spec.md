## ADDED Requirements

### Requirement: Full pool wait returns redis:unreachable
When every live socket is checked out, a further command SHALL wait for a slot. If no slot frees before the pool wait (200 milliseconds) elapses, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library (no extra timer goroutine leak: stop the timer when a slot arrives).

#### Scenario: Pool wait times out
- **WHEN** all live sockets at the default `poolSize` (8) are busy
- **AND** another command is issued
- **AND** no socket becomes free within 200 milliseconds
- **THEN** that command returns `redis:unreachable`
- **AND** the fake or server observes no additional TCP connection for that command

### Requirement: Live cap is proven on Redis and Dragonfly
The session SHALL keep at most `poolSize` live TCP connections (idle plus in use; const default 8) against a real Redis and a real Dragonfly. Fake-server tests MUST NOT be the only proof. Traefik local-plugin Pester on `/redis` and `/dragonfly` SHALL overlap requests long enough to contend for sockets, observe at most `poolSize` clients on that backend (default 8), and observe `redis:unreachable` when a waiter exceeds the pool wait. Compiled tests gated on live addresses SHALL prove pool-wait `redis:unreachable` on both engines; they MAY set unexported `poolSize` so a shared CI Redis is not left in Lua BUSY. Any Lua used to hold a socket MUST be Lua 5.1-safe (no `table.maxn`) and MUST list touched keys in KEYS (zero keys when none are touched). Compose project `reclaim-e2e`, routes `/a` `/b`, and existing verb headers MUST keep their semantics.

#### Scenario: Concurrent holds stay within poolSize on Redis
- **WHEN** overlapping requests through the Traefik plugin hold sockets against compose Redis
- **THEN** Redis has at most the default `poolSize` (8) established TCP clients from that plugin
- **AND** reclaim routes `/a` and `/b` still succeed

#### Scenario: Concurrent holds stay within poolSize on Dragonfly
- **WHEN** overlapping requests through the Traefik plugin hold sockets against compose Dragonfly
- **THEN** Dragonfly has at most the default `poolSize` (8) established TCP clients from that plugin

#### Scenario: Extra waiter is redis:unreachable on both engines
- **WHEN** `poolSize` sockets (default 8) are held against Redis
- **AND** another request cannot obtain a socket before the pool wait
- **THEN** that request fails with `redis:unreachable`
- **WHEN** the same overlap is run against Dragonfly
- **THEN** that request fails with `redis:unreachable`

## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most eight (`maxIdleConns`). Live sockets (idle plus checked out) SHALL not exceed `poolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `poolSize`, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `poolSize` connections, including when more callers overlap than `poolSize`. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one if live sockets are under `poolSize`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. `release` MUST NOT close a reusable socket solely because the idle list is full while live sockets are under `poolSize`.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Overlapping callers above poolSize do not dial past the live cap
- **WHEN** more concurrent commands than the default `poolSize` (8) overlap against a live fake Redis
- **THEN** the fake observes at most the default `poolSize` (8) TCP connections
- **AND** waiters reuse a released socket rather than dialing past `poolSize`
