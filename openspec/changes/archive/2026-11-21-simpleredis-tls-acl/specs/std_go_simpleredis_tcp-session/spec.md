## MODIFIED Requirements

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, user, password, database, a clone of the TLS configuration, pool knobs, timeout knobs, retry sentinels, and `Logger`) and SHALL create the in-use-turn channel sized to `PoolSize` (const default 8 when `PoolSize` is 0). `New` MUST NOT open a TCP connection. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, retry, or logger fields.

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

### Requirement: First command dials TCP
The first `Get`, `MGet` (with at least one name), `Set`, or `Del` after `New` SHALL dial `tcp` to the host stored by `New`. When the stored TLS configuration is nil the session SHALL speak RESP over that plain TCP connection. When it is non-nil the session SHALL complete a TLS client handshake with that configuration on that TCP connection before AUTH, SELECT, or the command. The session MUST NOT dial a Unix socket. The session MUST NOT build a server name from `Host`: when the stored configuration's `ServerName` is empty, `crypto/tls` takes it from the host part of the dial address. Zero-Config dial timeout SHALL be 200 milliseconds.

#### Scenario: Unreachable host
- **WHEN** a command is issued after `New` with host `127.0.0.1:1`
- **THEN** the command returns an error whose `Error()` text is `redis:unreachable`

#### Scenario: TLS round trip with an own CA
- **WHEN** `Config.TLS` trusts only the TLS fake's CA and `Host` is `localhost:<port>` of that fake
- **AND** `ServerName` is empty
- **THEN** `Set` then `Get` return the stored value
- **AND** the fake observes a completed TLS handshake before the first RESP command

#### Scenario: Client certificate for mutual TLS
- **WHEN** the TLS fake requires and verifies a client certificate
- **AND** `Config.TLS.Certificates` holds a certificate signed by the CA the fake trusts
- **THEN** a command succeeds
- **AND** the fake observes that client certificate

#### Scenario: TLS 1.2-only server
- **WHEN** the TLS fake allows at most TLS 1.2 and `Config.TLS` leaves `MinVersion` and `MaxVersion` unset
- **THEN** a command succeeds over TLS 1.2

#### Scenario: Nil TLS stays plain TCP
- **WHEN** `Config.TLS` is nil
- **THEN** the plain fake receives RESP as the first bytes on the connection

### Requirement: AUTH and SELECT run once per dial
When `user` or `pass` is non-empty, each new dial SHALL send `AUTH` before other commands. With a non-empty `user` the session SHALL send the two-argument form `AUTH <user> <pass>`, including `AUTH <user> ""` when `pass` is empty. With an empty `user` and a non-empty `pass` the session SHALL send the one-argument form `AUTH <pass>`. When both are empty the session MUST NOT send `AUTH`. When `database` is non-empty, each new dial SHALL send `SELECT` with that database before other commands. Subsequent commands on a reused connection MUST NOT send `AUTH` or `SELECT` again. AUTH and SELECT SHALL run after the TLS handshake when a TLS configuration is set.

#### Scenario: Auth and select once per dial
- **WHEN** the client is created with `New` with a password and a database
- **AND** several Gets reuse one connection
- **THEN** AUTH and SELECT are sent once for that connection
- **AND** they are not sent again on those Gets

#### Scenario: Password only keeps the one-argument form
- **WHEN** the client is created with `New` with `Pass` `secret` and an empty `User`
- **AND** a command is issued
- **THEN** the fake receives `AUTH secret` with exactly one argument

#### Scenario: ACL user sends the two-argument form
- **WHEN** the client is created with `New` with `User` `app` and `Pass` `secret`
- **AND** a command is issued
- **THEN** the fake receives `AUTH app secret` with exactly two arguments before the command

#### Scenario: ACL user without a password still authenticates as that user
- **WHEN** the client is created with `New` with `User` `app` and an empty `Pass`
- **AND** a command is issued
- **THEN** the fake receives `AUTH app ""` with exactly two arguments, the second empty

#### Scenario: Rejected ACL user maps to redis:noauth
- **WHEN** the client is created with `New` with `User` `app` and a wrong `Pass`
- **AND** the fake replies to AUTH with `-WRONGPASS invalid username-password pair or user is disabled.`
- **THEN** the command returns `redis:noauth`
- **AND** `errors.Is` matches `ErrNoAuth`
- **AND** the fake accepted one TCP connection
