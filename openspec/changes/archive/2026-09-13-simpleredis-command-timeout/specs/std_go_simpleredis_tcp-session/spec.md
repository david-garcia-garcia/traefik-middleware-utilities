## ADDED Requirements

### Requirement: The socket deadline is the remaining command budget
A command socket's `SetDeadline` SHALL be the time remaining on the command context, which `bindCommandDeadline` has already set to `now + CommandTimeout` or the caller's sooner deadline. There MUST NOT be a second, separate bound on the same socket: no per-operation cap and no per-kernel-`Read` deadline refresh. Mapping MUST keep `errors.Is` against `os.ErrDeadlineExceeded` and MUST NOT type-assert `net.Error`; because that deadline is the command deadline, a fired socket deadline SHALL report the context deadline and let the retry layer decide library (`redis:timeout`) versus caller (`Err()`). A compliant peer that keeps sending a bulk whose transfer needs more wall time than any single arrival window SHALL still return that value when the transfer finishes inside the budget. A peer that keeps sending slowly SHALL NOT hold an in-use turn past the budget; extra turn returns SHALL stay 0. The decoder's `maxBulkLength` cap SHALL stay a parse cap, not a time-derived size.

The accepted cost SHALL be that a peer which goes quiet mid-reply is not cut off early and instead ends the command when the budget runs out. Two bounds on one socket could only disagree, and the shorter of them made a large reply unreadable while the peer was healthy and still sending; a single bound is the resolution. Operators who need a shorter ceiling SHALL lower `CommandTimeout`, which is the ceiling itself.

#### Scenario: Streaming bulk whose transfer outlasts one arrival window returns intact
- **WHEN** a fake peer replies with a multi-megabyte bulk written in chunks with pauses between them
- **AND** that transfer still finishes inside `CommandTimeout`
- **AND** a Get is issued
- **THEN** the Get returns the payload bytes the peer sent
- **AND** the error is nil

#### Scenario: Silent mid-reply times out at the command budget
- **WHEN** a fake peer writes a bulk header and some payload bytes then sends nothing more
- **AND** a Get is issued
- **THEN** the command returns `redis:timeout`
- **AND** elapsed time is at least half of `CommandTimeout` and no more than `CommandTimeout` plus scheduling slack

#### Scenario: Drip peer cannot pin the in-use turn past the overall budget
- **WHEN** a fake peer sends bulk payload one small chunk at a time so bytes keep arriving but the whole transfer would exceed `CommandTimeout`
- **AND** a Get is issued
- **THEN** the command returns within `CommandTimeout` plus 50 milliseconds of scheduling slack
- **AND** extra turn returns are 0
- **AND** a later Get on that client can obtain a turn

## MODIFIED Requirements

### Requirement: Whole command has an overall deadline
Each command SHALL compute one overall deadline at entry equal to `now + CommandTimeout` unless the caller's context deadline is sooner. When the library instant is sooner, the command SHALL bind it onto the caller's context (`context.WithDeadline`, same shape as `net.Dialer` / `http.Client`). Every attempt, and the dial, AUTH, SELECT, and command I/O inside them, SHALL share that one budget. AUTH and SELECT MUST NOT each add a fresh timeout on top of a completed TCP connect. Per-command socket I/O SHALL set `SetDeadline` to the time remaining on that bound context. When that overall library deadline expires, the command SHALL return `redis:timeout` and MUST NOT start another attempt. A sooner caller deadline SHALL return that context's `Err()`.

`MaxRetries` MUST NOT be a multiplicand of the deadline: it SHALL bound the attempt count only. Retries SHALL stop when either the attempt count is exhausted or `CommandTimeout` expires, whichever comes first, so raising `MaxRetries` MUST NOT raise the latency ceiling. `DialTimeout` MUST NOT be folded into `CommandTimeout`'s value: it stays a per-dial-attempt cap, and a dial attempt is bounded by the smaller of the two.

#### Scenario: Black-hole Get returns within the overall deadline
- **WHEN** a zero-Config client Gets against `203.0.113.1:6379`
- **THEN** the command returns within `CommandTimeout` plus 50 milliseconds of scheduling slack
- **AND** the error `Error()` text is `redis:unreachable` or `redis:timeout`

#### Scenario: Handshake stall is bounded by the overall deadline
- **WHEN** `Pass` and `Database` are set
- **AND** the peer completes the TCP handshake and never replies
- **AND** a command is issued
- **THEN** the command returns within `CommandTimeout` plus 50 milliseconds of scheduling slack
- **AND** that elapsed time does not grow with the number of stalled handshake steps or attempts

#### Scenario: A high MaxRetries does not raise the ceiling
- **WHEN** `MaxRetries` is 10 and retry backoff is off
- **AND** the peer completes the TCP handshake and never replies
- **AND** a command is issued
- **THEN** the command returns within `CommandTimeout` plus 50 milliseconds of scheduling slack

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, password, database, pool knobs, timeout knobs, retry sentinels) and SHALL create the in-use-turn channel sized to `PoolSize` (const default 8 when `PoolSize` is 0). `New` MUST NOT open a TCP connection. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, or retry fields.

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` 200 milliseconds, `CommandTimeout` 900 milliseconds. Retry fields on `Config`: `0` at `New` means 1 extra retry; `-1` means off (one send). An explicit `MaxRetries` of 3 means 3 extra retries. `MinRetryBackoff` / `MaxRetryBackoff` keep go-redis sentinels: `0` means 8ms / 512ms; `-1` means off.

The session SHALL expose exactly two timeout knobs that bound a command, and each SHALL bound something individually. `DialTimeout` SHALL cap one TCP dial attempt (`net.Dialer.Timeout`); `CommandTimeout` SHALL cap the whole command. `CommandTimeout` MUST NOT be derived from `MaxRetries` or `DialTimeout`, and a knob that bounds no operation MUST NOT be exported.

`New` SHALL return `ErrMaxIdleConnsAbovePoolSize` and a nil client when an **explicit** `MaxIdleConns` is above `PoolSize`, and MUST NOT rewrite either knob in that case. A zero `MaxIdleConns` is not a request for 8: it SHALL take `min(8, PoolSize)`, so a `Config` that sets only a `PoolSize` below 8 SHALL still create a client whose `MaxIdleConns()` equals that `PoolSize`. `New` MUST NOT rewrite `PoolSize`. A trim below `PoolSize` SHALL still create a client.

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

### Requirement: I/O deadline is timeout, not a net.Error assert
When a command hits an I/O deadline, the session SHALL return an error whose `Error()` text is `redis:timeout`. Mapping MUST use `errors.Is` against `os.ErrDeadlineExceeded`. The session MUST NOT type-assert `net.Error` (Yaegi has panicked on that assert across the interpreter boundary). `redis:timeout` MUST NOT be retried (documented deviation from go-redis; that deadline is the remainder of `CommandTimeout`, 900 milliseconds by default, so a retry would have no time left to spend). A timeout on a reused connection MUST NOT open a second connection.

The deviation from go-redis is deliberate and larger than one default value. go-redis splits socket I/O into `ReadTimeout` (default 5 seconds) and `WriteTimeout` (follows `ReadTimeout`), each stamped once per command read or write as `min(now+timeout, ctx.Deadline())`, and has no whole-command bound: each retry attempt gets a fresh timeout and `MaxRetries` defaults to 3. It also offers escape hatches this session does not: `ReadTimeout: -1` means no timeout beyond the context, and `-2` skips `SetReadDeadline` entirely. Verified at pin `github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599` (`options.go` lines 744-758 and 697-698; `internal/pool/conn.go` lines 1155-1181 `WithReader` and 1374-1402 `deadline`). This session instead exports one `CommandTimeout` that bounds the whole command, because a Traefik middleware on the request path needs a latency ceiling an operator can read off the config rather than derive.

#### Scenario: I/O timeout is redis:timeout
- **WHEN** the Redis peer does not complete a reply before the I/O deadline
- **THEN** the command returns `redis:timeout`
