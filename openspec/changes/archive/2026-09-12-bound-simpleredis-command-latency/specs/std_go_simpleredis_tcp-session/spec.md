## MODIFIED Requirements

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, password, database, pool knobs, I/O knobs, retry sentinels) and SHALL create the in-use-turn channel sized to `PoolSize` (const default 8 when `PoolSize` is 0). `New` MUST NOT open a TCP connection. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, or retry fields.

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` 200 milliseconds, `IOTimeout` 100 milliseconds. Retry fields on `Config`: `0` at `New` means 1 extra retry; `-1` means off (one send). An explicit `MaxRetries` of 3 means 3 extra retries. `MinRetryBackoff` / `MaxRetryBackoff` keep go-redis sentinels: `0` means 8ms / 512ms; `-1` means off.

#### Scenario: New does not open a socket
- **WHEN** `New` is called with a host that refuses connections
- **THEN** `New` returns a client without error
- **AND** no TCP connection is opened

#### Scenario: Live cap is frozen at New
- **WHEN** `Config.PoolSize` is 1 at `New`
- **AND** `PoolSize` is written to 16 on that Config and on the client after `New`
- **AND** one command holds the only live turn
- **AND** another command waits past `PoolTimeout`
- **THEN** that waiter returns `redis:unreachable`
- **AND** the fake observes at most one TCP connection

### Requirement: First command dials TCP
The first `Get`, `MGet` (with at least one name), `Set`, or `Del` after `New` SHALL dial `tcp` to the host stored by `New`. The session MUST NOT dial a Unix socket and MUST NOT use TLS. Zero-Config dial timeout SHALL be 200 milliseconds.

#### Scenario: Unreachable host
- **WHEN** a command is issued after `New` with host `127.0.0.1:1`
- **THEN** the command returns an error whose `Error()` text is `redis:unreachable`

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (const default 8). Live sockets (idle plus checked out) SHALL not exceed `PoolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `PoolSize`, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `PoolSize` connections, including when more callers overlap than `PoolSize`. An idle connection older than `IdleTimeout` (const default thirty seconds) SHALL not be reused; the next command SHALL dial a new one if live sockets are under `PoolSize`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. `release` MUST NOT close a reusable socket solely because the idle list is full while live sockets are under `PoolSize`.

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

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). `redis:timeout` MUST NOT be retried (documented deviation from go-redis; `IOTimeout` default is 100 milliseconds). A timeout on a reused connection MUST NOT open a second connection.

#### Scenario: I/O timeout is redis:timeout
- **WHEN** the Redis peer does not complete a reply before the I/O deadline
- **THEN** the command returns `redis:timeout`

## ADDED Requirements

### Requirement: Whole command has an overall deadline
Each command SHALL compute one overall deadline at entry equal to `(maxRetries+1)*(DialTimeout+IOTimeout)` unless the caller's context deadline is sooner. Dial, AUTH, SELECT, and the command SHALL share the remaining time. AUTH and SELECT MUST NOT each add a fresh full `IOTimeout` on top of a completed TCP connect. When that overall deadline expires, the command SHALL return `redis:timeout` and MUST NOT start another attempt.

#### Scenario: Black-hole Get returns within the overall deadline
- **WHEN** a zero-Config client Gets against `203.0.113.1:6379`
- **THEN** the command returns within `(maxRetries+1)*(DialTimeout+IOTimeout)` plus 50 milliseconds of scheduling slack
- **AND** the error `Error()` text is `redis:unreachable` or `redis:timeout`

#### Scenario: Handshake stall is bounded by the overall deadline
- **WHEN** `Pass` and `Database` are set
- **AND** the peer completes the TCP handshake and never replies
- **AND** a command is issued
- **THEN** the command returns within the overall deadline
- **AND** elapsed time is less than `DialTimeout + 2×IOTimeout` times the number of attempts that would fit if each step used a fresh `IOTimeout`

### Requirement: Command context can cancel the session
A command that takes a context SHALL return that context's `Err()` when the context is cancelled or its deadline is exceeded. The session SHALL close the in-use socket rather than return it to the idle pool, and SHALL free the in-use turn. Mapping MUST NOT type-assert `net.Error`. A cancelled wait for a pool turn SHALL NOT acquire a turn.

#### Scenario: Cancel mid-command returns promptly and frees the turn
- **WHEN** a command is in flight against a peer that does not reply
- **AND** the caller cancels the context
- **THEN** the command returns that context's `Err()`
- **AND** a later command on the same client can obtain a pool turn
- **AND** the cancelled command's socket is not in the idle pool
