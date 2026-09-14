## Purpose

Defines the structured slog lines a SimpleRedis client emits so operators can diagnose AUTH failure, pool pressure, protocol poison, over-free, and panic without logging secrets or Redis keys.

## Requirements

### Requirement: Ad-hoc slog at decision sites
The session SHALL call `*slog.Logger` methods at the decision site with an inline `simpleredis_` message string. The session MUST NOT export message-name constants and MUST NOT add a logging helper type or file. Each emit SHALL be slog key/value attributes at Error, Warn, or Debug. The session MUST NOT emit an Info-level event. `simpleredis_socket_poisoned` SHALL fire at the post-reply leftover check in `do`. `simpleredis_auth_leftover` SHALL fire at the pre-write leftover check in `do` when the next verb is AUTH or SELECT.

`simpleredis_timeout` SHALL fire where the command budget is classified as `redis:timeout`, which is the overall-deadline mapping in `exec`, not in `do`. `do` MUST NOT emit it: the socket deadline is reported upwards as a context deadline so the overall-deadline owner decides library budget versus caller deadline, so a `do`-side emit would either never fire or duplicate that one.

#### Scenario: Panic is Error then re-raised
- **WHEN** `do` panics inside `runOnConn`
- **THEN** the client emits `simpleredis_panic` at Error
- **AND** the panic still reaches the caller
- **AND** the in-use turn is returned, the socket is closed, and `OverFrees()` is `0`

#### Scenario: AUTH rejection is Error
- **WHEN** AUTH is rejected with `WRONGPASS`, `NOAUTH`, `NOPERM`, or `ERR Client sent AUTH`
- **THEN** the client emits `simpleredis_noauth` at Error

#### Scenario: Over-free is Error
- **WHEN** an extra in-use-turn return is dropped because the semaphore is already full
- **THEN** the client emits `simpleredis_over_free` at Error

#### Scenario: Pool wait is Warn
- **WHEN** a waiter exceeds `PoolTimeout`
- **THEN** the client emits `simpleredis_pool_exhausted` at Warn

#### Scenario: Short bulk is Warn
- **WHEN** the peer announced more bulk payload than it wrote
- **THEN** the client emits `simpleredis_short_bulk` at Warn

#### Scenario: Malformed RESP is Warn
- **WHEN** the peer sends malformed or unsupported RESP (`redis:issue?` or `redis:unsupported-reply`)
- **THEN** the client emits `simpleredis_bad_reply` at Warn

#### Scenario: Post-reply leftover is Warn
- **WHEN** unread RESP remains in the connection reader after a complete reply
- **THEN** the client emits `simpleredis_socket_poisoned` at Warn

#### Scenario: Handshake leftover is Warn
- **WHEN** unread bytes remain in the reader before AUTH or SELECT is written
- **THEN** the client emits `simpleredis_auth_leftover` at Warn

#### Scenario: Non-auth handshake failure is Warn
- **WHEN** AUTH or SELECT fails for a reason that is not AUTH-class (`LOADING`, max clients)
- **THEN** the client emits `simpleredis_handshake_failed` at Warn
- **AND** that line carries `verb` `AUTH` or `SELECT`, and no attribute carrying the peer's error text

#### Scenario: An AUTH reply that echoes the password is not logged
- **WHEN** the peer answers AUTH with `ERR AUTH <password> called without any password configured for the default user` (Redis 7.4 against a nopass default user, which is not an AUTH-class prefix)
- **THEN** `simpleredis_handshake_failed` is emitted at Warn without that text
- **AND** the captured output does not contain the password
- **AND** the caller's returned error still carries the peer's full text

#### Scenario: Debug events fire on their paths
- **WHEN** a logger enabled at Debug is set on `Config` at `New`
- **AND** the client dials, sweeps idle, retries, times out, is canceled, resolves MSetEX native vs Lua, reloads a script after NOSCRIPT, or is constructed
- **THEN** it emits the matching `simpleredis_dial`, `simpleredis_idle_swept`, `simpleredis_retry`, `simpleredis_timeout`, `simpleredis_canceled`, `simpleredis_capability`, `simpleredis_noscript`, or `simpleredis_open` at Debug

### Requirement: Dial names why it dialled
`simpleredis_dial` SHALL carry a `reason` attribute. `reason` SHALL be `idle_miss` when the borrow consulted the unused-socket list and had nothing young to reuse, and `skip_idle` when the borrow bypassed that list because this command already failed I/O on a socket it took from it. Operators diagnose a peer restart, a failover, or a `CLIENT KILL` of the accepted sockets from a burst of `skip_idle`, and pool pressure from a burst of `idle_miss`; a dial event that does not separate those two is not enough to tell them apart. The session MUST NOT reuse one reason string for both cases.

#### Scenario: Bypassing a dropped unused list says skip_idle
- **WHEN** the unused list is warmed, every accepted socket at the peer is then dropped, and a command borrows one of those sockets and retries
- **THEN** a `simpleredis_dial` line carries `reason` `skip_idle`
- **AND** a `simpleredis_dial` line from warming the unused list carries `reason` `idle_miss`

### Requirement: Logs never include secrets or Redis keys
No slog line at any level SHALL include `Config.Pass`, a Redis key name, or a Redis value. Attribute `host` SHALL be the server address `New` froze from `Config.Host` when it is logged. An attribute MUST NOT carry a peer error text that can embed the credential the client sent; an event on the AUTH path SHALL name the step instead, since the caller's returned error already carries the peer text.

#### Scenario: Distinctive password and key never appear
- **WHEN** commands run with a distinctive password and a distinctive key name
- **AND** every slog line is captured at the most verbose level
- **THEN** neither string appears in the captured output

### Requirement: Interpreted logging works
Interpreted code SHALL construct a client with a `*slog.Logger` on `Config` and observe at least one `simpleredis_` message. Those tests MUST use GOPATH with stdlib symbols only. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi emits an open event
- **WHEN** interpreted code calls `New` with a capturing logger
- **THEN** the logger records `simpleredis_open`
