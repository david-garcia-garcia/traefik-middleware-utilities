## ADDED Requirements

### Requirement: Lost in-use turns MUST NOT brick the client
When a command takes an in-use turn and never returns it (a panic between take and return that Traefik recovers), the client SHALL NOT stay unable to dial for the process lifetime. The next waiter that would return pool-wait `redis:unreachable` SHALL restore missing turns up to `PoolSize` when the client owns zero sockets: unused idle sockets plus sockets a still-running command has checked out. Recovery MUST NOT restore turns while owned sockets are at `PoolSize`. `LostTurns()` SHALL increment by the number of tokens restored. `LostTurns()` SHALL be readable next to `OverFrees()`, `PoolSize()`, and `MaxIdleConns()`. `exec` MUST NOT defer `release` as the recovery. `New` MUST NOT start a goroutine to recover turns.

#### Scenario: Recovered panics after borrow do not brick the client
- **WHEN** a client of `PoolSize` 2 has warmed one idle socket
- **AND** `PoolSize` callers each take a turn via `borrow` then panic with `recover()` in the caller and never `release`
- **AND** a later Get runs
- **THEN** that Get succeeds
- **AND** `LostTurns()` is at least `PoolSize`

#### Scenario: Recovery does not fire while sockets are busy
- **WHEN** all live sockets at `PoolSize` 2 are busy in-flight commands
- **AND** another command is issued
- **AND** no socket becomes free within one `PoolTimeout`
- **THEN** that command returns `redis:unreachable`
- **AND** the fake observes no additional TCP connection for that command
- **AND** `LostTurns()` is 0

#### Scenario: LostTurns is zero when borrow and release stay balanced
- **WHEN** a client runs a sequential Get against a live fake
- **THEN** `LostTurns()` is 0
- **AND** the in-use-turn channel `len` equals `cap`
- **AND** `OverFrees()` is 0

## MODIFIED Requirements

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (const default 8). Live sockets (idle plus checked out) SHALL not exceed `PoolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `PoolSize`, a caller SHALL wait for a released socket instead of dialing. Live sockets the client owns are unused idle sockets plus sockets a still-running command has checked out. An empty in-use-turn channel with zero owned sockets is a leaked turn, not a full pool: the next waiter SHALL restore missing turns up to `PoolSize` and MAY dial. Recovery MUST NOT run while owned sockets are at `PoolSize`. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `PoolSize` connections, including when more callers overlap than `PoolSize`. An idle connection older than `IdleTimeout` (const default thirty seconds) SHALL not be reused. On the next command that borrows, every idle socket older than `IdleTimeout` SHALL be closed, including those behind a younger tail that is reused. If no younger idle socket remains, the next command SHALL dial a new one if live sockets are under `PoolSize`. `New` MUST NOT start a goroutine to close idle sockets. A client that issues no later command MAY keep idle sockets past `IdleTimeout` until `Close`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. `release` MUST NOT close a reusable socket solely because the idle list is full while live sockets are under `PoolSize`.

Every command (GET, MGET, SET, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL) SHALL use go-redis-shaped command retry. `MaxRetries`, `MinRetryBackoff`, and `MaxRetryBackoff` live on `Config` and are copied at `New`. The retry loop SHALL be `for attempt := 0; attempt <= maxRetries; attempt++` (zero Config means 1 extra retry, at most two sends). Backoff SHALL wait only between retries (`attempt > 0`), using go-redis `RetryBackoff` (`math/rand` `Int63n` jitter), and MUST return when the command context is done or the overall command deadline has passed. Construction is `New(Config)`.

A command SHALL retry when the error is `redis:unreachable` (EOF, unexpected EOF, dial failure, and other IO as this client maps them), including a fresh dial, unless the client is `Close`d (`redis:unreachable` from a closed client MUST NOT spin `MaxRetries`). A command SHALL also retry Redis error replies whose text is `ERR max number of clients reached` or has prefix `LOADING `, `READONLY `, `MASTERDOWN `, `CLUSTERDOWN `, or `TRYAGAIN ` (space after the word). A command MUST NOT retry `redis:timeout` (documented deviation from go-redis: `IOTimeout` default is 100 milliseconds), `redis:miss`, `redis:noauth`, `redis:issue?`, a cancelled or deadline-exceeded command context, or other Redis `-ERR` replies. A pool-wait timeout SHALL return `redis:unreachable` and MUST NOT be retried, so `MaxRetries` does not multiply `poolTimeout`.

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

#### Scenario: Stale idle head is closed while the tail stays hot
- **WHEN** a client has two idle sockets
- **AND** only the older socket is past `IdleTimeout`
- **AND** a command is issued
- **THEN** the fake observes that aged socket closed
- **AND** the command reuses the younger socket
- **AND** the fake does not accept a third TCP connection for that command
- **AND** `New` did not start a goroutine

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

### Requirement: Full pool wait returns redis:unreachable
When every live socket the client owns is checked out, a further command SHALL wait for an in-use turn. If no turn frees before the pool wait (200 milliseconds) elapses, and the client still owns live sockets at `PoolSize`, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library (no extra timer goroutine leak: stop the timer when a turn arrives). That timeout MUST NOT be retried. The pool-wait sentinel SHALL wrap the unreachable sentinel so `errors.Is` matches unreachable, and the retry classifier MUST still treat pool wait as not retryable (`shouldRetry` false for pool wait, true for a plain unreachable sentinel). Identity compare, or a pool-wait check before unreachable, is required; rewriting the classifier to `errors.Is` against unreachable alone MUST NOT retry pool wait.

#### Scenario: Pool wait times out
- **WHEN** all live sockets at the default `poolSize` (8) are busy
- **AND** another command is issued
- **AND** no socket becomes free within 200 milliseconds
- **THEN** that command returns `redis:unreachable`
- **AND** the fake or server observes no additional TCP connection for that command

#### Scenario: Pool wait is not retried after wrapping unreachable
- **WHEN** the error is the pool-wait sentinel
- **THEN** command retry does not retry that error
- **WHEN** the error is the plain unreachable sentinel
- **THEN** command retry does retry that error
