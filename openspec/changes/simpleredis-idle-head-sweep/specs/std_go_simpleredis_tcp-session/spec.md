## ADDED Requirements

### Requirement: Live idle-head tests run on Redis and Dragonfly in CI
Live tests SHALL table-drive Redis and Dragonfly addresses from `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or an address is missing. CI MUST start both engines, set both addresses, and MUST NOT skip those tests. Those tests MUST live in package `simpleredis` so they can observe idle-pool membership without a production `Reset` or `Dump`. They SHALL use `Get` (or another existing non-Eval verb). Traefik, Pester, compose, and Yaegi MUST NOT be the idle-head proof. If a later test adds Eval, that Eval MUST be Lua 5.1-safe and MUST list every touched key in KEYS.

#### Scenario: Both engines close a stale idle head
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **AND** the idle list has a connection older than thirty seconds at the head and a younger connection at the tail
- **AND** a Get runs
- **THEN** that Get succeeds against Redis
- **AND** the stale head socket is closed and is not left in the idle list
- **AND** the same assertions pass against Dragonfly

## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most eight. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than eight connections. An idle connection older than thirty seconds SHALL not be reused; the next command SHALL dial a new one. A dead pooled connection SHALL be retried once unless the error is a timeout.

When a reusable connection is released, the session SHALL close idle-head entries older than thirty seconds before appending that connection or closing it for a full pool or a closed client. The session SHALL stop at the first idle-head entry that is still inside thirty seconds. The session MUST NOT inspect past that still-valid head into the hot tail. The session MUST NOT start a background reaper goroutine. `idleTimeout` SHALL remain thirty seconds. `maxIdleConns` SHALL remain eight.

#### Scenario: Sequential gets reuse one connection
- **WHEN** twenty-five Gets run one after another against a live fake Redis
- **THEN** the fake observes one TCP connection

#### Scenario: Concurrent commands stay within the pool
- **WHEN** eight concurrent commands run against a live fake Redis
- **THEN** the fake observes at most eight TCP connections

#### Scenario: Stale idle head is closed while the tail stays hot
- **WHEN** the idle list has two connections against a live fake Redis
- **AND** the head connection is older than thirty seconds
- **AND** the tail connection is still inside thirty seconds
- **AND** a Get runs
- **THEN** the stale head socket is closed
- **AND** that socket is not left in the idle list
- **AND** the Get succeeds
