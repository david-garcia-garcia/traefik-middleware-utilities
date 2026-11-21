## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (const default 8). Live sockets (idle plus checked out) SHALL not exceed `PoolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `PoolSize`, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `PoolSize` connections, including when more callers overlap than `PoolSize`. An idle connection older than `IdleTimeout` (const default thirty seconds) SHALL not be reused; the next command SHALL dial a new one if live sockets are under `PoolSize`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. When unused sockets already equal `MaxIdleConns`, `release` SHALL close a reusable socket instead of growing the idle pool, even while live sockets are under `PoolSize`. After a burst of returns, unused sockets SHALL equal `min(MaxIdleConns, PoolSize)` and the peer's still-open sockets SHALL equal that idle count.

Every command (GET, MGET, SET, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL) SHALL use go-redis-shaped command retry. `MaxRetries`, `MinRetryBackoff`, and `MaxRetryBackoff` live on `Config` and are copied at `New`. The retry loop SHALL be `for attempt := 0; attempt <= maxRetries; attempt++` (default 3 means at most four sends). Backoff SHALL sleep only between retries (`attempt > 0`), using go-redis `RetryBackoff` (`math/rand` `Int63n` jitter). Construction is `New(Config)`.

A command SHALL retry when the error is `redis:unreachable` (EOF, unexpected EOF, dial failure, and other IO as this client maps them), including a fresh dial, unless the client is `Close`d (`redis:unreachable` from a closed client MUST NOT spin `MaxRetries`). A command SHALL also retry Redis error replies whose text is `ERR max number of clients reached` or has prefix `LOADING `, `READONLY `, `MASTERDOWN `, `CLUSTERDOWN `, or `TRYAGAIN ` (space after the word). A command MUST NOT retry `redis:timeout` (documented deviation from go-redis: `ioTimeout` is one second; retrying would stall a Traefik request for several seconds), `redis:miss`, `redis:noauth`, `redis:issue?`, or other Redis `-ERR` replies. A pool-wait timeout SHALL return `redis:unreachable` and MUST NOT be retried, so `MaxRetries` does not multiply `poolTimeout`.

INCR, INCRBY, and EVAL MAY double-apply when a reply is lost and the command is retried. That is accepted. A timeout on a reused connection MUST NOT be retried for any verb.

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

#### Scenario: Sequential returns honour MaxIdleConns below PoolSize
- **WHEN** `PoolSize` sockets are checked out then returned one by one against a fake Redis
- **AND** `PoolSize` is 8 and `MaxIdleConns` is 2
- **THEN** unused sockets equal 2 after the last return
- **AND** after waiting for peer close, still-open sockets equal 2
- **WHEN** `PoolSize` is 16 and `MaxIdleConns` is 1
- **THEN** unused sockets equal 1 after the last return
- **AND** after waiting for peer close, still-open sockets equal 1

#### Scenario: Sequential returns honour MaxIdleConns at or above PoolSize
- **WHEN** `PoolSize` sockets are checked out then returned one by one against a fake Redis
- **AND** `PoolSize` is 2 and `MaxIdleConns` is 8
- **THEN** unused sockets equal 2 after the last return
- **AND** after waiting for peer close, still-open sockets equal 2
- **WHEN** `PoolSize` is 8 and `MaxIdleConns` is 8
- **THEN** unused sockets equal 8 after the last return
- **AND** after waiting for peer close, still-open sockets equal 8

#### Scenario: Concurrent commands quiesce at MaxIdleConns
- **WHEN** more concurrent Gets than `MaxIdleConns` overlap against a fake Redis
- **AND** `PoolSize` is 16 and `MaxIdleConns` is the default 8
- **THEN** after those Gets finish, unused sockets equal 8
- **AND** after waiting for peer close, still-open sockets equal 8
- **AND** a later Get reuses an idle socket and the fake's accept count does not increase

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
