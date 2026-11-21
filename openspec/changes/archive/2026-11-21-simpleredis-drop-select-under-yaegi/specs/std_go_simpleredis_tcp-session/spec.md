## ADDED Requirements

### Requirement: Session source contains no select statement
Non-test SimpleRedis session source MUST NOT contain a Go `select` statement. Yaegi v0.16.1 shares one select-case array per `select` statement across every goroutine that runs it, so a `select` on per-call or per-client channels can wait on, or receive from, another goroutine's channels. A unit test SHALL parse every non-test `.go` file of the package and SHALL fail naming the file and line of each `select` it finds.

#### Scenario: Guard passes on select-free session source
- **WHEN** the unit suite parses every non-test `.go` file in `simpleredis/`
- **THEN** no `select` statement is found
- **AND** at least one file was scanned

#### Scenario: Guard names a reintroduced select
- **WHEN** a non-test session file contains a `select` statement
- **THEN** the guard test fails
- **AND** its message names that file and line

### Requirement: Interpreted concurrency does not cross turns or deadlines
Under Yaegi v0.16.1, concurrent session calls SHALL behave as they do compiled: each pool wait, retry backoff, and in-use-turn take or return SHALL act only on its own call and client. Proof SHALL run the session's own code under Yaegi inside `go test`, each case beside a compiled control that runs the same probe. Interpreted cases MAY skip when the race detector is on; compiled controls MUST NOT.

#### Scenario: Saturated wait under Yaegi returns its own outcome
- **WHEN** interpreted callers wait for the only in-use turn of a client while it stays held, some with a live context and some with a 1 millisecond context
- **THEN** no call returns no socket together with no error
- **AND** each live-context caller returns the pool-wait error no earlier than its own `PoolTimeout`
- **AND** each short-context caller returns its context error
- **AND** a caller whose context is cancelled returns that context's error before `PoolTimeout`
- **AND** no call is still blocked after the last deadline

#### Scenario: Retry backoff under Yaegi wakes on its own delay
- **WHEN** many interpreted goroutines back off 1 millisecond on a 300 millisecond context while others back off 600 milliseconds on a live context
- **THEN** every 1 millisecond backoff returns no error and does not sit until its context deadline
- **AND** no 600 millisecond backoff returns before its own delay

#### Scenario: Two interpreted clients keep their own turns
- **WHEN** two interpreted clients each serve concurrent Gets without saturating their pools
- **THEN** after the callers return, each client's in-use-turn semaphore is full
- **AND** each client's `OverFrees()` is 0

#### Scenario: Interpreted saturated Get does not panic
- **WHEN** many interpreted goroutines call `Get` on a client with a small `PoolSize`, a short `PoolTimeout`, and contexts of a few milliseconds
- **THEN** the interpreter does not panic
- **AND** every error is the pool-wait error, a context error, or another error the compiled suite already classifies for that path
- **AND** after the callers return the in-use-turn semaphore is full and `OverFrees()` is 0

## MODIFIED Requirements

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, user, password, database, a clone of the TLS configuration, pool knobs, timeout knobs, retry sentinels, and `Logger`) and SHALL create the in-use-turn semaphore with its cap and its free-turn count both set to `PoolSize` (const default 8 when `PoolSize` is 0). The in-use-turn semaphore is **full** when its free-turn count equals its cap. `New` MUST NOT open a TCP connection. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, retry, or logger fields.

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` 200 milliseconds, `CommandTimeout` 900 milliseconds. Retry fields on `Config`: `0` at `New` means 1 extra retry; `-1` means off (one send). An explicit `MaxRetries` of 3 means 3 extra retries. `MinRetryBackoff` / `MaxRetryBackoff` keep go-redis sentinels: `0` means 8ms / 512ms; `-1` means off.

The session SHALL expose exactly two timeout knobs that bound a command, and each SHALL bound something individually. `DialTimeout` SHALL cap one dial attempt as a whole: the TCP connect, plus the TLS handshake when a TLS configuration is set (`net.Dialer.Timeout` on the `tls.Dialer`'s `NetDialer`); `CommandTimeout` SHALL cap the whole command. `CommandTimeout` MUST NOT be derived from `MaxRetries` or `DialTimeout`, and a knob that bounds no operation MUST NOT be exported. The session MUST NOT add a separate handshake timeout knob.

`New` SHALL return `ErrMaxIdleConnsAbovePoolSize` and a nil client when an **explicit** `MaxIdleConns` is above `PoolSize`, and MUST NOT rewrite either knob in that case. A zero `MaxIdleConns` is not a request for 8: it SHALL take `min(8, PoolSize)`, so a `Config` that sets only a `PoolSize` below 8 SHALL still create a client whose `MaxIdleConns()` equals that `PoolSize`. `New` MUST NOT rewrite `PoolSize`. A trim below `PoolSize` SHALL still create a client.

A nil `Config.Logger` SHALL be replaced at `New` with a discard logger (`slog.NewTextHandler` writing to `io.Discard`) so later call sites never nil-check the logger. A nil `Logger` MUST NOT be a reason for `New` to return an error. `log/slog` is a Go standard-library package and SHALL be an allowed import of the session source.

A nil `Config.TLS` SHALL mean plain TCP. A non-nil `Config.TLS` SHALL be stored as `(*tls.Config).Clone()` of the caller's value, so later writes to that value's fields MUST NOT change the client. The clone is shallow: values the caller mutates in place behind a shared pointer or slice (`RootCAs` pool contents, `Certificates` elements, callbacks) remain the caller's responsibility. `New` MUST NOT reject, clear, or wrap callbacks on that configuration, and MUST NOT return an error because of `TLS` or `User`. `crypto/tls` is a Go standard-library package and SHALL be an allowed import of the session source.

#### Scenario: Zero Config timeout knobs
- **WHEN** a client is created with `New` from a `Config` whose only field is `Host`
- **THEN** `DialTimeout()` is 200 milliseconds
- **AND** `CommandTimeout()` is 900 milliseconds

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

#### Scenario: Nil logger is silent and New still succeeds
- **WHEN** `New` is called with a zero `Config` except `Host`
- **THEN** `New` returns a client without error
- **AND** later commands on that client MUST NOT panic for want of a logger

#### Scenario: TLS configuration is frozen at New
- **WHEN** `New` is called with a `Config.TLS` whose `RootCAs` trusts the TLS fake's CA and whose `ServerName` is `localhost`
- **AND** after `New` the caller sets that `tls.Config`'s `ServerName` to `wrong.example` and its `RootCAs` to an empty pool
- **AND** a command is issued against the TLS fake whose certificate names `localhost`
- **THEN** the command succeeds

#### Scenario: Handshake time counts against DialTimeout
- **WHEN** `Config.TLS` is set and `DialTimeout` is 200 milliseconds
- **AND** the peer accepts TCP and never answers the ClientHello
- **THEN** each dial attempt ends at about 200 milliseconds, not at `CommandTimeout`

#### Scenario: New fills the in-use-turn semaphore
- **WHEN** `New` is called with `PoolSize` 3
- **THEN** the in-use-turn semaphore cap is 3
- **AND** the in-use-turn semaphore is full

### Requirement: Full pool wait returns redis:unreachable
When every live socket is checked out, a further command SHALL wait for an in-use turn. If no turn frees before the pool wait (200 milliseconds) elapses, that command SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT open another TCP connection. The wait MUST use only the Go standard library and MUST NOT leak its timeout or its cancel wake-up: when a turn arrives, both SHALL be released. A turn returned while a command waits SHALL be handed to a waiting command; a waiter that gives up on timeout or cancel MUST NOT keep a turn and MUST NOT cause a turn to be lost. That timeout MUST NOT be retried. The pool-wait sentinel SHALL wrap the unreachable sentinel so `errors.Is` matches unreachable, and the retry classifier MUST still treat pool wait as not retryable (`shouldRetry` false for pool wait, true for a plain unreachable sentinel). Identity compare, or a pool-wait check before unreachable, is required; rewriting the classifier to `errors.Is` against unreachable alone MUST NOT retry pool wait.

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

#### Scenario: Returned turn goes to a waiter
- **WHEN** a client with `PoolSize` 1 has its only turn held
- **AND** another command is waiting for a turn
- **AND** the holder returns its turn before `PoolTimeout`
- **THEN** the waiting command obtains that turn
- **AND** after it finishes the in-use-turn semaphore is full

#### Scenario: Racing handoff and give-up neither lose nor duplicate turns
- **WHEN** many commands wait on a saturated client while holders return turns and waiters time out or are cancelled at about the same instant
- **THEN** every waiter either obtains a turn or returns the pool-wait error or its context error
- **AND** after all callers return the in-use-turn semaphore is full
- **AND** `OverFrees()` is 0

### Requirement: Client not from New fails immediately
A command on a `SimpleRedis` that did not come from `New` SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT be retried (MUST NOT sleep retry backoff). `New` remains the only constructor of the in-use-turn semaphore. `Close` on that client SHALL be idempotent and MUST NOT panic. Empty `MGet` and empty or mismatched `MSetEX` / `MSetEXAt` stay pre-dial validation and MUST NOT panic.

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
- **THEN** that client's in-use-turn semaphore cap is 0

### Requirement: Peer-closed idle vintage is recovered sequentially
When every unused pooled TCP connection is closed by the peer at once (restart, failover, or kill of every accepted socket) while those sockets are still younger than `IdleTimeout`, sequential commands SHALL succeed while the peer is accepting, using the existing `MaxRetries` policy. An I/O failure on a socket taken from idle MUST map to `redis:unreachable`. The next attempt of that command SHALL be a new dial, not another unused pooled socket. A later sequential command MAY still take one leftover dead unused socket and then force-dial on its own retry. A force-dial that itself fails with `redis:unreachable` SHALL consume `MaxRetries` as dest already does. When `MaxRetries` is `-1` (one send), that command SHALL still fail after one unused-socket I/O; a free extra send MUST NOT be added, because a lost reply on a reused socket is indistinguishable from a dead unused socket after a successful write. Timeouts, handshake AUTH or SELECT failures, pool wait, and a closed client MUST NOT skip idle. The in-use-turn semaphore SHALL stay full at rest with `OverFrees()` equal to 0. Compiled proof MUST warm unused sockets with simultaneous in-flight commands, close every accepted socket from the server, then issue sequential commands; it MUST NOT close the client-side file descriptors.

The unused pool exists so that a sequential burst on one client pays one dial plus one AUTH/SELECT instead of one per command, and reacting to a failed borrow is the price of that reuse. A borrow-time liveness probe of the shape go-redis uses (`syscall.Conn` to `RawConn.Read` to a non-blocking `syscall.Read`, where `EAGAIN` means healthy and zero bytes means the peer is gone) MUST NOT be adopted to avoid this vintage. Traefik registers `syscall` symbols only when `useUnsafe` is true on **both** the plugin manifest and the operator's static config, and manifest-true with operator-false makes Traefik refuse to load the plugin at all, which a middleware other operators install cannot require. That probe is also Unix-only, and Yaegi v0.16.1 does not evaluate `//go:build` lines, so a `conn_check.go` / `conn_check_dummy.go` split of the kind go-redis ships loads both files and silently resolves to the no-op. The session SHALL therefore detect a dead unused socket by using it.

#### Scenario: Sequential Gets succeed after every idle socket is dropped
- **WHEN** `PoolSize` idle sockets have been warmed with simultaneous in-flight Gets
- **AND** the fake closes every accepted socket while remaining up and accepting
- **AND** sequential Gets are issued one after another with the zero-Config `MaxRetries` default
- **THEN** those Gets succeed
- **AND** none return `redis:unreachable` while the peer is accepting

#### Scenario: Default pool sequential recovery
- **WHEN** `PoolSize` is 8 and `MaxRetries` is the zero-Config default
- **AND** every idle socket is closed from the server
- **THEN** the next sequential Get succeeds

#### Scenario: Turns stay balanced after vintage recovery
- **WHEN** sequential Gets have recovered after every idle socket was dropped
- **THEN** the in-use-turn semaphore is full
- **AND** `OverFrees()` is 0

### Requirement: Extra in-use turn return MUST NOT hang
When a caller returns an in-use turn while the in-use-turn semaphore is already full (free turns equal the cap after `New`) and no command is waiting, that return SHALL complete without waiting. The return that would exceed the cap MUST be dropped. `OverFrees()` SHALL increment by one for that drop. `OverFrees()` SHALL be readable next to `PoolSize()` and `MaxIdleConns()`. Balanced borrow and release SHALL leave the in-use-turn semaphore full and `OverFrees()` equal to 0. The client MUST NOT panic on an extra return.

#### Scenario: Extra return on a fresh client returns promptly
- **WHEN** a client is created with `New` so the in-use-turn semaphore is full
- **AND** one extra in-use turn is returned without a matching take
- **THEN** that return completes without waiting
- **AND** `OverFrees()` is 1
- **AND** the in-use-turn semaphore free count still equals its cap

#### Scenario: Hammered borrow and release stay balanced
- **WHEN** many goroutines hammer borrow and release through a healthy fake, a dead address, an AUTH-rejecting fake, and a starved pool with a short `PoolTimeout`
- **THEN** those goroutines all finish
- **AND** the in-use-turn semaphore is full
- **AND** `OverFrees()` is 0

### Requirement: Chaos pool stays within live cap and does not mix keys
Compiled unit tests SHALL drive concurrent Gets against a peer that, per command, randomly replies honestly, delays a few milliseconds, closes with no reply, replies LOADING, or writes a short bulk then closes. After those callers stop, at rest the in-use-turn semaphore SHALL be full, extra turn returns SHALL be zero, and settled server-side open sockets SHALL be at most PoolSize. A non-error Get MUST return that key’s own value. Tests MUST NOT assert a peak live-socket count. Tests that sample idle length then the in-use-turn free count as a live-socket metric are invalid. `go test -short` MUST skip this stress.

#### Scenario: Chaos Get does not return another key
- **WHEN** many goroutines Get keys k0..k63 against that chaotic peer for a bounded duration
- **THEN** every Get that returns no error returns that key’s own value
- **AND** after quiescence the in-use-turn semaphore is full
- **AND** extra turn returns are zero
- **AND** settled server-side open sockets are at most PoolSize

#### Scenario: Short skips chaos stress
- **WHEN** `go test -short` runs the package
- **THEN** the chaos pool stress does not run

### Requirement: Close cycles do not leak goroutines or sockets
Compiled unit tests SHALL run many New / use / Close cycles and SHALL prove goroutine count is flat and server-side open sockets are zero. When N Gets are held, Close, then those Gets are released, idle SHALL be empty, server-side open sockets SHALL be zero, and the in-use-turn semaphore SHALL be full. `go test -short` MUST skip this stress.

#### Scenario: Repeated New use Close leaves no sockets
- **WHEN** a test constructs a client, issues commands, and Closes, many times
- **THEN** after the last Close, server-side open sockets are zero
- **AND** goroutine count is not higher than before the cycles

#### Scenario: Close during held Gets returns every turn
- **WHEN** N Gets are held at the peer, the client is Closed, and the holds are released
- **THEN** idle is empty
- **AND** server-side open sockets are zero
- **AND** the in-use-turn semaphore is full

### Requirement: Panic between borrow and release returns the turn and closes the socket
When a command panics after the session has taken an in-use turn and before that turn is returned, the session SHALL still return the turn and SHALL close that socket. The panicked socket MUST NOT return to the idle pool. After `PoolSize` recovered panics on one client, a later command on that client SHALL still obtain a turn against a healthy peer. Extra turn returns SHALL stay 0. Session source MUST NOT grow leak-detection or turn-refill counters.

A panic inside the session's command I/O SHALL be treated the same as a dirty reply: the socket is destroyed. A panic while obtaining a socket, after the turn is taken and before the socket is handed to the caller, SHALL return the turn without leaving a live socket checked out.

Compiled proof MUST panic inside command I/O while the TCP socket remains a healthy peer the session can close. Interpreted proof MUST show that a deferred restore runs under Yaegi v0.16.1 for an explicit interpreted panic, the interpreter `errors.As` panic on a package-local struct, and a nil-map write.

#### Scenario: Recovered panics return every turn and the client still serves
- **WHEN** a client is created with `PoolSize` 2
- **AND** one idle socket is warmed
- **AND** two commands panic inside command I/O after taking a turn, and the caller recovers each panic
- **THEN** after those panics the in-use-turn semaphore is full
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
- **AND** the in-use-turn semaphore is not short that turn
- **AND** extra turn returns stay 0

#### Scenario: Full idle closes then the turn is returned
- **WHEN** a reusable socket is released while idle is already at `MaxIdleConns`
- **THEN** that socket is closed, not parked
- **AND** the in-use-turn is returned after the idle-list mutex is released
- **AND** extra turn returns stay 0
