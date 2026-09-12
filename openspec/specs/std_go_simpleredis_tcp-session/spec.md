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

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` two seconds, `IOTimeout` one second. Retry fields on `Config` follow go-redis Options sentinels: `0` means the default (3 extra retries, 8ms, 512ms); `-1` means off.

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
The first `Get`, `MGet` (with at least one name), `Set`, or `Del` after `New` SHALL dial `tcp` to the host stored by `New`. The session MUST NOT dial a Unix socket and MUST NOT use TLS. Dial timeout SHALL be two seconds.

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
The session SHALL keep unused TCP connections in an idle pool of at most `MaxIdleConns` (const default 8). Live sockets (idle plus checked out) SHALL not exceed `PoolSize` (`liveCap()`; const default 8). When idle is empty and live sockets are already at `PoolSize`, a caller SHALL wait for a released socket instead of dialing. A sequential burst of Gets on one client SHALL reuse one connection. Concurrent commands SHALL not open more than `PoolSize` connections, including when more callers overlap than `PoolSize`. An idle connection older than `IdleTimeout` (const default thirty seconds) SHALL not be reused; the next command SHALL dial a new one if live sockets are under `PoolSize`. Dirty sockets MUST NOT return to idle (`reusable=false`); that is not a retry. `release` MUST NOT close a reusable socket solely because the idle list is full while live sockets are under `PoolSize`.

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

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). `redis:timeout` MUST NOT be retried (documented deviation from go-redis; `IOTimeout` default is one second). A timeout on a reused connection MUST NOT open a second connection.

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
When AUTH on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool. AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. When SELECT on a new dial returns an error, the session SHALL close that socket and MUST NOT append it to the idle pool, and SHALL return that error text. `ERR DB index is out of range` MUST NOT map to `redis:noauth`. AUTH SHALL run before SELECT when both password and database are non-empty. A handshake failure SHALL surface one error to the caller and MUST NOT open a second TCP connection for that command. In-process handshake-failure tests SHALL use a fake whose AUTH and SELECT replies are configurable (default success so existing success tests stay). Live Redis and Dragonfly tests SHALL prove the cases each dest engine supports and SHALL skip when those engines are unset.

#### Scenario: Fake AUTH rejected maps to redis:noauth and is not pooled
- **WHEN** the client is created with `New` with a non-empty password and an empty database
- **AND** the fake replies to AUTH with an AUTH-class prefix (`NOAUTH`, `WRONGPASS`, `NOPERM`, or `ERR Client sent AUTH`)
- **AND** a command is issued
- **THEN** the command returns `redis:noauth`
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

### Requirement: Peer-closed idle socket is retried
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry on a new dial under the go-redis-shaped `MaxRetries` policy. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against both Redis and Dragonfly, then the next command SHALL succeed on a new dial. Those live tests MUST skip under `-short` or when both SimpleRedis live addresses are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. The nested Traefik plugin SHALL keep `simpleredis.New` in Traefik `New`. A recover request (`recover=1`) SHALL run Set and Get only, SHALL set `X-SimpleRedis-Recover: ok` when those succeed after recovery, and MUST NOT Eval. Default `/redis` and `/dragonfly` verb headers MUST stay. Existing Eval on the default path SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

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
### Requirement: Lost-reply Incr and Eval are proven on live Redis and Dragonfly
A request through the nested SimpleRedis Traefik plugin SHALL, in addition to the happy-path verbs, send Incr and Eval through a compose RESP drop-relay in front of that request’s engine (`redis:6379` or `dragonfly:6379`). The drop-relay SHALL drop INCR, INCRBY, or EVAL only after that TCP session has already forwarded at least one command (the probe warms with GET). A retry on a new session whose first command is INCR or EVAL SHALL pass the reply through. Other verbs SHALL pass through. The plugin SHALL set `X-SimpleRedis-DropIncr` to the integer after two applies (`2`), `X-SimpleRedis-DropIncrStored` to the stored value `2` read from the engine (not the drop-relay), `X-SimpleRedis-DropEval` to the integer after two applies of the Kong script (`6` when ARGV INCRBY is `3`), and `X-SimpleRedis-DropEvalStored` to that stored script result. Eval SHALL use a Lua 5.1-safe script that lists its key in KEYS. Happy-path Host stays `redis:6379` / `dragonfly:6379`. The Redis and Dragonfly Pester Describes MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Pester asserts lost-reply Incr and Eval on Redis
- **WHEN** a request is made on `/redis`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts lost-reply Incr and Eval on Dragonfly
- **WHEN** a request is made on `/dragonfly`
- **THEN** `X-SimpleRedis-DropIncr` is `2`
- **AND** `X-SimpleRedis-DropIncrStored` is `2`
- **AND** `X-SimpleRedis-DropEval` is `6`
- **AND** `X-SimpleRedis-DropEvalStored` is `6`
- **AND** the Dragonfly Pester Describe does not stop `whoami-a` or `whoami-b`

