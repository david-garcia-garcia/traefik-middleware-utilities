## Purpose

Defines the stdlib TCP session a SimpleRedis client holds: `New(Config)` copies settings without dialing; the first command dials; AUTH and SELECT run once per dial; idle connections are pooled; `Close` drains the pool and blocks further dials. Callers import `simpleredis` from this module. The session loads under Traefik Yaegi from a nested fake plugin that calls `simpleredis.New` in Traefik `New`.

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

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, password, database, pool knobs, I/O knobs, retry sentinels) and SHALL create the in-use-turn channel sized to `PoolSize` (const default 8 when `PoolSize` is 0). `New` MUST NOT open a TCP connection. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, or retry fields.

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` 200 milliseconds, `IOTimeout` 100 milliseconds. Retry fields on `Config`: `0` at `New` means 1 extra retry; `-1` means off (one send). An explicit `MaxRetries` of 3 means 3 extra retries. `MinRetryBackoff` / `MaxRetryBackoff` keep go-redis sentinels: `0` means 8ms / 512ms; `-1` means off.

`New` SHALL return `ErrMaxIdleConnsAbovePoolSize` and a nil client when an **explicit** `MaxIdleConns` is above `PoolSize`, and MUST NOT rewrite either knob in that case. A zero `MaxIdleConns` is not a request for 8: it SHALL take `min(8, PoolSize)`, so a `Config` that sets only a `PoolSize` below 8 SHALL still create a client whose `MaxIdleConns()` equals that `PoolSize`. `New` MUST NOT rewrite `PoolSize`. A trim below `PoolSize` SHALL still create a client.

#### Scenario: New does not open a socket
- **WHEN** `New` is called with a host that refuses connections
- **AND** pool knobs are valid after defaults
- **THEN** `New` returns a client and a nil error
- **AND** no TCP connection is opened

#### Scenario: New rejects an explicit MaxIdleConns above PoolSize
- **WHEN** `New` is called with `PoolSize` 4 and `MaxIdleConns` 100
- **THEN** `New` returns no client
- **AND** the error is `ErrMaxIdleConnsAbovePoolSize`
- **WHEN** `New` is called with `PoolSize` 4 and `MaxIdleConns` 5
- **THEN** `New` returns no client
- **AND** the error is `ErrMaxIdleConnsAbovePoolSize`

#### Scenario: A defaulted MaxIdleConns follows a smaller PoolSize
- **WHEN** `New` is called with `PoolSize` 2 and `MaxIdleConns` left at 0
- **THEN** `New` returns a client
- **AND** `MaxIdleConns()` is 2
- **WHEN** `New` is called with `PoolSize` 1 and `MaxIdleConns` left at 0
- **THEN** `New` returns a client
- **AND** `MaxIdleConns()` is 1

#### Scenario: Live cap is frozen at New
- **WHEN** `Config.PoolSize` is 1 at `New` and `MaxIdleConns` is 1
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

### Requirement: AUTH and SELECT run once per dial
When `pass` is non-empty, each new dial SHALL send `AUTH` with that password before other commands. When `database` is non-empty, each new dial SHALL send `SELECT` with that database before other commands. Subsequent commands on a reused connection MUST NOT send `AUTH` or `SELECT` again.

#### Scenario: Auth and select once per dial
- **WHEN** the client is created with `New` with a password and a database
- **AND** several Gets reuse one connection
- **THEN** AUTH and SELECT are sent once for that connection
- **AND** they are not sent again on those Gets

### Requirement: Idle connections are pooled
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (const default 8). Live sockets (idle plus checked out) SHALL not exceed `PoolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `PoolSize`, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `PoolSize` connections, including when more callers overlap than `PoolSize`. An idle connection older than `IdleTimeout` (const default thirty seconds) SHALL not be reused. On the next command that borrows, every idle socket older than `IdleTimeout` SHALL be closed, including those behind a younger tail that is reused. If no younger idle socket remains, the next command SHALL dial a new one if live sockets are under `PoolSize`. `New` MUST NOT start a goroutine to close idle sockets. A client that issues no later command MAY keep idle sockets past `IdleTimeout` until `Close`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. When unused sockets already equal `MaxIdleConns`, `release` SHALL close a reusable socket instead of growing the idle pool, even while live sockets are under `PoolSize`. After a burst of returns, unused sockets SHALL equal `min(MaxIdleConns, PoolSize)` and the peer's still-open sockets SHALL equal that idle count.

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

#### Scenario: Sequential returns honour MaxIdleConns below PoolSize
- **WHEN** `PoolSize` sockets are checked out then returned one by one against a fake Redis
- **AND** `PoolSize` is 8 and `MaxIdleConns` is 2
- **THEN** unused sockets equal 2 after the last return
- **AND** after waiting for peer close, still-open sockets equal 2
- **WHEN** `PoolSize` is 16 and `MaxIdleConns` is 1
- **THEN** unused sockets equal 1 after the last return
- **AND** after waiting for peer close, still-open sockets equal 1

#### Scenario: Sequential returns honour MaxIdleConns equal to PoolSize
- **WHEN** `PoolSize` sockets are checked out then returned one by one against a fake Redis
- **AND** `PoolSize` is 8 and `MaxIdleConns` is 8
- **THEN** unused sockets equal 8 after the last return
- **AND** after waiting for peer close, still-open sockets equal 8

#### Scenario: Concurrent commands quiesce at MaxIdleConns
- **WHEN** more concurrent Gets than `MaxIdleConns` overlap against a fake Redis
- **AND** `PoolSize` is 16 and `MaxIdleConns` is the default 8
- **THEN** after those Gets finish, unused sockets equal 8
- **AND** after waiting for peer close, still-open sockets equal 8
- **AND** a later Get reuses an idle socket and the fake's accept count does not increase

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
When every live socket is checked out, a further command SHALL wait for an in-use turn. If no turn frees before the pool wait (200 milliseconds) elapses, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library (no extra timer goroutine leak: stop the timer when a turn arrives). That timeout MUST NOT be retried. The pool-wait sentinel SHALL wrap the unreachable sentinel so `errors.Is` matches unreachable, and the retry classifier MUST still treat pool wait as not retryable (`shouldRetry` false for pool wait, true for a plain unreachable sentinel). Identity compare, or a pool-wait check before unreachable, is required; rewriting the classifier to `errors.Is` against unreachable alone MUST NOT retry pool wait.

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

### Requirement: Live cap is proven on Redis and Dragonfly
The session SHALL keep at most `poolSize` live TCP connections (idle plus in use; const default 8) against a real Redis and a real Dragonfly. Fake-server tests MUST NOT be the only proof. Traefik local-plugin Pester on `/redis` and `/dragonfly` SHALL overlap requests long enough to contend for sockets, observe at most `poolSize` clients on that backend (default 8), and observe `redis:unreachable` when a waiter exceeds the pool wait. Compiled tests gated on live addresses SHALL prove pool-wait `redis:unreachable` on both engines; they MAY set `Config.PoolSize` so a shared CI Redis is not left in Lua BUSY. Those compiled tests MUST skip under `-short` or when both `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY` are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. Any Lua used to hold a socket MUST be Lua 5.1-safe (no `table.maxn`) and MUST list touched keys in KEYS (zero keys when none are touched). Compose project `reclaim-e2e`, routes `/a` `/b`, and existing verb headers MUST keep their semantics.

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
### Requirement: Close drains the pool and blocks redial
`Close` SHALL close idle pooled connections and mark the client closed. After `Close`, `Get`, `MGet`, `Set`, and `Del` SHALL return `redis:unreachable` and MUST NOT dial. `Close` SHALL be idempotent. In-flight commands MAY finish; their sockets SHALL be closed on release.

#### Scenario: Close drains idle and does not redial
- **WHEN** a client has an idle pooled connection
- **AND** `Close` is called
- **AND** a later Get is issued
- **THEN** that Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

#### Scenario: Close during an in-flight command closes the socket on release
- **WHEN** a command is in flight
- **AND** `Close` is called
- **AND** that command then finishes
- **THEN** idle is empty
- **AND** that command's socket is closed
- **AND** a later Get returns `redis:unreachable`
- **AND** no new TCP connection is opened

### Requirement: Client not from New fails immediately
A command on a `SimpleRedis` that did not come from `New` SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT be retried (MUST NOT sleep retry backoff). `New` remains the only constructor of the in-use-turn channel. `Close` on that client SHALL be idempotent and MUST NOT panic. Empty `MGet` and empty or mismatched `MSetEX` / `MSetEXAt` stay pre-dial validation and MUST NOT panic.

#### Scenario: Zero-value Get does not retry
- **WHEN** `Get` is called on `&SimpleRedis{}`
- **THEN** the command returns `redis:unreachable`
- **AND** it returns in under the default minimum retry backoff (8 milliseconds)

#### Scenario: Zero-value exported methods do not panic
- **WHEN** every exported command and `Close` is called on `&SimpleRedis{}` (non-empty `MGet` / `MSetEX` / `MSetEXAt` so they reach the session)
- **AND** `Close` is called a second time
- **THEN** no call panics
- **AND** each command that reaches the session returns `redis:unreachable`

#### Scenario: New remains the only in-use-turn constructor
- **WHEN** a client is `&SimpleRedis{}` and `New` has not run
- **THEN** that client has no in-use-turn channel

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). `redis:timeout` MUST NOT be retried (documented deviation from go-redis; `IOTimeout` default is 100 milliseconds). A timeout on a reused connection MUST NOT open a second connection.

#### Scenario: I/O timeout is redis:timeout
- **WHEN** the Redis peer does not complete a reply before the I/O deadline
- **THEN** the command returns `redis:timeout`

### Requirement: Library New loads under Traefik Yaegi
A Traefik local plugin SHALL import this module’s `simpleredis` package. Traefik `New` SHALL call `simpleredis.New` only (no command, so Traefik still starts if Redis is late). Traefik SHALL start. A request through that plugin SHALL succeed. `useunsafe` MUST be false. That plugin MUST be a nested module, not this repo’s root `plugin.go`. The existing reclaim e2e compose project, Traefik container, ports 8000/8080, and routes `/a` `/b` MUST keep their reclaim semantics.

#### Scenario: Fake plugin starts without dialing
- **WHEN** Traefik v3.7.11 loads a local plugin whose `New` calls `simpleredis.New`
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
When AUTH on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool. AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth` and SHALL match `errors.Is(err, ErrNoAuth)` for both compiled and Yaegi-interpreted callers. When SELECT on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool, and SHALL return that error text. `ERR DB index is out of range` MUST NOT map to `redis:noauth`. AUTH SHALL run before SELECT when both password and database are non-empty. A handshake failure SHALL surface one error to the caller and MUST NOT open a second TCP connection for that command. That MUST NOT-redial rule includes AUTH or SELECT peer close with no reply, AUTH `-LOADING Redis is loading the dataset in memory`, and AUTH `-ERR max number of clients reached`. AUTH or SELECT peer close SHALL return `redis:unreachable` and SHALL match `IsUnreachable` for both compiled and Yaegi-interpreted callers. The session MUST NOT wrap that error in a package-local type whose `Unwrap` compiled `errors.Is` cannot see under Yaegi. TCP dial refuse before AUTH/SELECT MAY still retry. A command that receives `LOADING ` after a successful handshake MAY still retry. In-process handshake-failure tests SHALL use a fake whose AUTH and SELECT replies are configurable (default success so existing success tests stay). Live Redis and Dragonfly tests SHALL prove the cases each dest engine supports and SHALL skip when those engines are unset.

#### Scenario: Fake AUTH rejected maps to redis:noauth and is not pooled
- **WHEN** the client is created with `New` with a non-empty password and an empty database
- **AND** the fake replies to AUTH with an AUTH-class prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, or `ERR Client sent AUTH`)
- **AND** a command is issued
- **THEN** the command returns `redis:noauth`
- **AND** `errors.Is` matches `ErrNoAuth` for a compiled caller
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake SELECT rejected after AUTH is not pooled
- **WHEN** the client is created with `New` with a password and database `99`
- **AND** the fake replies `+OK` to AUTH and `-ERR DB index is out of range` to SELECT
- **AND** a command is issued
- **THEN** AUTH is sent before SELECT
- **AND** the command returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **AND** the fake observes that the client closed the socket
- **AND** the fake accepted one TCP connection

#### Scenario: Fake AUTH close without reply is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the peer accepts TCP, reads AUTH, and closes with no reply
- **AND** a command is issued
- **THEN** the command returns `redis:unreachable`
- **AND** `IsUnreachable` is true for a compiled caller
- **AND** the peer accepted one TCP connection

#### Scenario: Fake SELECT close without reply after AUTH is not redialed
- **WHEN** the client is created with `New` with a password, a database, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the peer accepts TCP, replies `+OK` to AUTH, reads SELECT, and closes with no reply
- **AND** a command is issued
- **THEN** the command returns `redis:unreachable`
- **AND** `IsUnreachable` is true for a compiled caller
- **AND** the peer accepted one TCP connection

#### Scenario: Fake AUTH LOADING is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the fake replies to AUTH with `-LOADING Redis is loading the dataset in memory`
- **AND** a command is issued
- **THEN** the command returns an error whose text is `LOADING Redis is loading the dataset in memory`
- **AND** the fake accepted one TCP connection

#### Scenario: Fake AUTH max-clients is not redialed
- **WHEN** the client is created with `New` with a non-empty password, `MaxRetries: 1`, and MinRetryBackoff off
- **AND** the fake replies to AUTH with `-ERR max number of clients reached`
- **AND** a command is issued
- **THEN** the command returns an error whose text is `ERR max number of clients reached`
- **AND** the fake accepted one TCP connection

#### Scenario: Live SELECT 99 on Redis and Dragonfly
- **WHEN** dest Redis and Dragonfly are reachable without a password
- **AND** the client is created with `New` with database `99`
- **AND** a command is issued
- **THEN** each engine returns `ERR DB index is out of range`
- **AND** the idle pool is empty
- **WHEN** those engines are unset
- **THEN** the live SELECT tests skip

#### Scenario: Live wrong password on Redis and Dragonfly with requirepass
- **WHEN** dest Redis and Dragonfly are reachable with requirepass set
- **AND** the client is created with `New` with a wrong password
- **AND** a command is issued
- **THEN** each engine returns `redis:noauth`
- **AND** `errors.Is` matches `ErrNoAuth`
- **AND** the idle pool is empty
- **WHEN** those passworded engines are unset
- **THEN** the live wrong-password tests skip

### Requirement: Dirty reply is not returned to the idle pool
When a command’s reply is a short bulk read (the peer announces more payload bytes than it writes before closing), the session SHALL NOT return that socket to the idle pool. That discard is not a retry. When `MaxRetries` is off (`-1`), a short bulk read SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT return `redis:issue?`. After that failed command, a later command on the same client SHALL return the value for its own key; the discarded socket MUST NOT leak a prior payload. Truncated-payload coverage MUST be a unit test against a peer that can write raw bytes and close mid-stream; it MUST NOT depend on live Redis or Dragonfly emitting a lying length. After `readLine` uses `ReadSlice` for the RESP head, the short-read path is still `io.ReadFull` of the announced bulk payload; the unit fake MUST still announce `$100`, write 40 bytes, and close so that payload `ReadFull` fails. Other malformed type bytes, missing CR, unparseable lengths, and illegal array-element heads are specified on `std_go_simpleredis_resp-commands`.

#### Scenario: Truncated bulk is unreachable and not pooled
- **WHEN** `MaxRetries` is `-1`
- **AND** a Get receives `$100\r\n`, then 40 bytes, then a close
- **THEN** the command returns `redis:unreachable`
- **AND** the error is not `redis:issue?`
- **AND** the idle pool is empty after that call

#### Scenario: Second Get after truncate returns its own value
- **WHEN** that truncated Get has returned
- **AND** a later Get is issued for a key whose next reply is a complete bulk of known bytes
- **THEN** that Get returns those bytes
- **AND** the idle pool was empty after the truncated call

### Requirement: Leftover unread reply is not returned to the idle pool
When a command’s reply is one complete RESP value and unread bytes remain in that connection’s reader at the reply boundary, the session SHALL return that decoded value to the caller and SHALL NOT return that socket to the idle pool. The session MUST NOT drain leftover bytes to resynchronise. This requirement covers leftover already pulled into the reader. It does not cover an unsolicited reply that arrives only into the kernel receive buffer while the socket is idle. A later command on the same client SHALL dial a new connection when the idle pool is empty after that discard. A sequential burst of Gets against a peer that writes exactly one complete reply per command SHALL still reuse one connection. When unread bytes remain in the reader before the next command is written on the same connection (AUTH then SELECT on a newly dialed socket), the session SHALL NOT write that next command on that socket. That pre-write refusal SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT return `redis:issue?`, same as a short bulk read: the peer damaged the protocol, the socket is destroyed and not pooled, and the command fails. A handshake AUTH leftover that trips that refusal SHALL still surface one error and MUST NOT open a second TCP connection for that command (handshake failure is not retried).

#### Scenario: Stray extra bulk is not pooled
- **WHEN** `MaxRetries` is `-1`
- **AND** `PoolSize` is `1`
- **AND** a Get receives a complete bulk for its own key plus one extra well-formed bulk
- **THEN** that Get returns the bytes for its own key
- **AND** the idle pool is empty after that call

#### Scenario: Next command after leftover dials a new connection
- **WHEN** that leftover Get has returned
- **AND** a later Get is issued for another key against a peer that writes one complete bulk per command
- **THEN** that Get returns the bytes for its own key
- **AND** the peer accepted a new TCP connection for that later Get

#### Scenario: AUTH leftover is unreachable and SELECT is not written
- **WHEN** `MaxRetries` is `-1`
- **AND** the client is created with a password and a database
- **AND** AUTH receives a complete `+OK` plus one extra well-formed bulk
- **THEN** the command returns `redis:unreachable`
- **AND** the error is not `redis:issue?`
- **AND** SELECT was not written
- **AND** the idle pool is empty

#### Scenario: AUTH leftover is not retried on a second dial
- **WHEN** `MaxRetries` is `1`
- **AND** AUTH leftover is as in the previous scenario
- **THEN** the fake accepted exactly one TCP connection
- **AND** AUTH ran once and SELECT was not written

#### Scenario: Compliant sequential Gets reuse one connection
- **WHEN** a client issues 25 sequential Gets against a peer that writes exactly one complete reply per command
- **THEN** the peer accepted exactly one TCP connection

### Requirement: Peer-closed idle socket is retried
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry on a new dial under the go-redis-shaped `MaxRetries` policy. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against both Redis and Dragonfly, then the next command SHALL succeed on a new dial. Those live tests MUST skip under `-short` or when both SimpleRedis live addresses are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. The nested Traefik plugin SHALL keep `simpleredis.New` in Traefik `New`. Pester recover SHALL be health warmup, `CLIENT KILL`, then Set and Get only, and MUST NOT Eval. Exact `/redis` and `/dragonfly` SHALL remain health Set+Get. Verb paths live on `/redis/<verb>` and `/dragonfly/<verb>`. Eval on `/eval` SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Peer-closed idle is retried
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
- **THEN** the next Get is still retried
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
- **AND** a later Set then Get is made on `/redis`
- **THEN** the Get status is 200
- **AND** the Get body is the value Set wrote
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Traefik Dragonfly recover after kill
- **WHEN** a request has already succeeded on `/dragonfly`
- **AND** the probe's pooled Dragonfly connection is killed with `CLIENT KILL` by `ADDR` or `ID`
- **AND** a later Set then Get is made on `/dragonfly`
- **THEN** the Get status is 200
- **AND** the Get body is the value Set wrote
- **AND** the SimpleRedis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Default verb paths stay
- **WHEN** Pester GETs `/redis/get` after Set
- **THEN** the response body is the value Set wrote
- **AND** POST `/redis/eval` lists its key in `KEYS` and is Lua 5.1-safe

### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, with query `drop=1` on `/redis` and `/dragonfly` verb paths, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL drop INCR, INCRBY, or EVAL only after that TCP session has already forwarded at least one command (Pester warms with GET). A retry on a new session whose first command is INCR or EVAL SHALL pass the reply through. Other verbs SHALL pass through. After drop Incr the body SHALL be the integer after two applies (`2`) and a Get without `drop=1` SHALL return stored `2`. After drop Eval of the Kong script (ARGV INCRBY `3`) the body SHALL be `6` and a Get without `drop=1` SHALL return stored `6`. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** Pester Incrs and Evals `/redis` with `drop=1` after a Get warmup
- **THEN** the Incr body is `2`
- **AND** a Get without `drop=1` of that key is `2`
- **AND** the Eval body is `6`
- **AND** a Get without `drop=1` of that Eval key is `6`
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** Pester Incrs and Evals `/dragonfly` with `drop=1` after a Get warmup
- **THEN** the Incr body is `2`
- **AND** a Get without `drop=1` of that key is `2`
- **AND** the Eval body is `6`
- **AND** a Get without `drop=1` of that Eval key is `6`
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`

### Requirement: Extra in-use turn return MUST NOT hang
When a caller returns an in-use-turn token while the in-use-turn channel is already full (`len` equals `cap` after `New`), that return SHALL complete without waiting. The send that would block MUST be dropped. `OverFrees()` SHALL increment by one for that drop. `OverFrees()` SHALL be readable next to `PoolSize()` and `MaxIdleConns()`. Balanced borrow and release SHALL leave the in-use-turn channel full (`len` equals `cap`) and `OverFrees()` equal to 0. The client MUST NOT panic on an extra return.

#### Scenario: Extra return on a fresh client returns promptly
- **WHEN** a client is created with `New` so the in-use-turn channel is full
- **AND** one extra in-use turn is returned without a matching take
- **THEN** that return completes without waiting
- **AND** `OverFrees()` is 1

#### Scenario: Hammered borrow and release stay balanced
- **WHEN** many goroutines hammer borrow and release through a healthy fake, a dead address, an AUTH-rejecting fake, and a starved pool with a short `PoolTimeout`
- **THEN** those goroutines all finish
- **AND** the in-use-turn channel `len` equals `cap`
- **AND** `OverFrees()` is 0

### Requirement: Whole command has an overall deadline
Each command SHALL compute one overall deadline at entry equal to `(maxRetries+1)*(DialTimeout+IOTimeout)` unless the caller's context deadline is sooner. When the library instant is sooner, the command SHALL bind it onto the caller's context (`context.WithDeadline`, same shape as `net.Dialer` / `http.Client`). Dial, AUTH, SELECT, and the command SHALL share the remaining time. AUTH and SELECT MUST NOT each add a fresh full `IOTimeout` on top of a completed TCP connect. Per-command socket I/O SHALL still `SetDeadline` to the lesser of `IOTimeout` and time remaining. When that overall library deadline expires, the command SHALL return `redis:timeout` and MUST NOT start another attempt. A sooner caller deadline SHALL return that context's `Err()`.

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

### Requirement: Session source keeps copy conversions
The SimpleRedis session source SHALL convert command names, scripts, and decimal arguments with `[]byte(...)` and integer-reply payloads with `string(...)`. It MUST NOT add `unsafe` zero-copy helpers in session source. It MUST NOT import `unsafe` or use cgo. Traefik local-plugin `useunsafe` MUST stay false.

#### Scenario: Copy conversions stay in session source
- **WHEN** the session source converts a string key, script, or decimal argument to bytes, or a bulk integer payload to a string
- **THEN** that conversion is `[]byte(...)` or `string(...)`
- **AND** session source has no unsafe pointer or header cast that aliases string and `[]byte`

### Requirement: Chaos pool stays within live cap and does not mix keys
Compiled unit tests SHALL drive concurrent Gets against a peer that, per command, randomly replies honestly, delays a few milliseconds, closes with no reply, replies LOADING, or writes a short bulk then closes. After those callers stop, at rest the in-use-turn channel SHALL be full, extra turn returns SHALL be zero, and settled server-side open sockets SHALL be at most PoolSize. A non-error Get MUST return that key’s own value. Tests MUST NOT assert a peak live-socket count. Tests that sample idle length then in-use-turn length as a live-socket metric are invalid. `go test -short` MUST skip this stress.

#### Scenario: Chaos Get does not return another key
- **WHEN** many goroutines Get keys k0..k63 against that chaotic peer for a bounded duration
- **THEN** every Get that returns no error returns that key’s own value
- **AND** after quiescence the in-use-turn channel is full
- **AND** extra turn returns are zero
- **AND** settled server-side open sockets are at most PoolSize

#### Scenario: Short skips chaos stress
- **WHEN** `go test -short` runs the package
- **THEN** the chaos pool stress does not run

### Requirement: Close cycles do not leak goroutines or sockets
Compiled unit tests SHALL run many New / use / Close cycles and SHALL prove goroutine count is flat and server-side open sockets are zero. When N Gets are held, Close, then those Gets are released, idle SHALL be empty, server-side open sockets SHALL be zero, and the in-use-turn channel SHALL be full. `go test -short` MUST skip this stress.

#### Scenario: Repeated New use Close leaves no sockets
- **WHEN** a test constructs a client, issues commands, and Closes, many times
- **THEN** after the last Close, server-side open sockets are zero
- **AND** goroutine count is not higher than before the cycles

#### Scenario: Close during held Gets returns every turn
- **WHEN** N Gets are held at the peer, the client is Closed, and the holds are released
- **THEN** idle is empty
- **AND** server-side open sockets are zero
- **AND** the in-use-turn channel is full

### Requirement: Panic between borrow and release returns the turn and closes the socket
When a command panics after the session has taken an in-use turn and before that turn is returned, the session SHALL still return the turn and SHALL close that socket. The panicked socket MUST NOT return to the idle pool. After `PoolSize` recovered panics on one client, a later command on that client SHALL still obtain a turn against a healthy peer. Extra turn returns SHALL stay 0. Session source MUST NOT grow leak-detection or turn-refill counters.

A panic inside the session's command I/O SHALL be treated the same as a dirty reply: the socket is destroyed. A panic while obtaining a socket, after the turn is taken and before the socket is handed to the caller, SHALL return the turn without leaving a live socket checked out.

Compiled proof MUST panic inside command I/O while the TCP socket remains a healthy peer the session can close. Interpreted proof MUST show that a deferred restore runs under Yaegi v0.16.1 for an explicit interpreted panic, the interpreter `errors.As` panic on a package-local struct, and a nil-map write.

#### Scenario: Recovered panics return every turn and the client still serves
- **WHEN** a client is created with `PoolSize` 2
- **AND** one idle socket is warmed
- **AND** two commands panic inside command I/O after taking a turn, and the caller recovers each panic
- **THEN** after those panics the in-use-turn channel `len` equals `cap`
- **AND** idle is empty
- **AND** extra turn returns are 0
- **AND** a later Get on that client returns the stored value

#### Scenario: Interpreted defer restores on panic
- **WHEN** interpreted code takes a turn counter, registers a deferred restore, then panics by explicit panic, by `errors.As` on a package-local struct, or by a nil-map write
- **AND** the compiled caller recovers Traefik-style
- **THEN** the deferred restore has run for each of those three panics
- **AND** the turn counter is 0

### Requirement: Panic inside idle keep-or-close still unlocks the idle list
When the session decides whether a reusable socket returns to the idle pool, that decision SHALL run under the idle-list mutex, and that mutex SHALL be released even if the decision panics. The session MUST NOT hold that mutex across a socket close or an in-use-turn return. The in-use-turn return SHALL run after that decision returns, on every path, including when the socket is not reusable. Extra turn returns SHALL stay 0. Session source MUST NOT add a production hook solely to panic inside that locked decision.

The close decision SHALL stay the idle-only rule already required above: close when the client is closed, or when the idle list is already at `MaxIdleConns`. Otherwise park. `lastUsed` SHALL still be stamped on the reusable path before the socket is parked, including when the later verdict is close.

#### Scenario: Reusable socket parks then the turn is returned
- **WHEN** a reusable socket is released while idle is under `MaxIdleConns`
- **THEN** that socket is in the idle list
- **AND** `lastUsed` was stamped before it was parked
- **AND** the in-use-turn channel is not short that turn
- **AND** extra turn returns stay 0

#### Scenario: Full idle closes then the turn is returned
- **WHEN** a reusable socket is released while idle is already at `MaxIdleConns`
- **THEN** that socket is closed, not parked
- **AND** the in-use-turn is returned after the idle-list mutex is released
- **AND** extra turn returns stay 0

