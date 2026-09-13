## Purpose

Defines the optional structured slog events a SimpleRedis client emits so operators can diagnose AUTH failure, pool pressure, protocol poison, over-free, and panic without logging secrets or Redis keys.

## ADDED Requirements

### Requirement: Events use reclaim-shaped slog constants
The session SHALL export message-name constants whose values are `simpleredis_` event strings (`simpleredis_panic`, `simpleredis_noauth`, `simpleredis_not_from_new`, `simpleredis_over_free`, `simpleredis_pool_exhausted`, `simpleredis_short_bulk`, `simpleredis_bad_reply`, `simpleredis_handshake_failed`, `simpleredis_dial`, `simpleredis_idle_swept`, `simpleredis_retry`, `simpleredis_timeout`, `simpleredis_canceled`, `simpleredis_socket_closed`, `simpleredis_capability`, `simpleredis_noscript`, `simpleredis_open`, `simpleredis_close`). Each emit SHALL be slog key/value attributes at Error, Warn, or Debug. The session MUST NOT emit an Info-level event. Constants `MsgSocketPoisoned` (`simpleredis_socket_poisoned`) and `MsgAuthLeftover` (`simpleredis_auth_leftover`) MAY be declared; this change MUST NOT invent leftover-RESP detection in order to fire them.

#### Scenario: Panic is Error then re-raised
- **WHEN** `do` panics inside `runOnConn`
- **THEN** the client emits `simpleredis_panic` at Error with attributes `panic`, `stack`, and `host`
- **AND** the panic still reaches the caller
- **AND** the in-use turn is returned, the socket is closed, and `OverFrees()` is `0`

#### Scenario: AUTH rejection is Error
- **WHEN** AUTH is rejected with `WRONGPASS`, `NOAUTH`, `NOPERM`, or `ERR Client sent AUTH`
- **THEN** the client emits `simpleredis_noauth` at Error with attributes `host` and `error`

#### Scenario: Zero-value client is Error
- **WHEN** a command runs on a client that did not come from `New`
- **THEN** the client emits `simpleredis_not_from_new` at Error with attribute `host`

#### Scenario: Over-free is Error
- **WHEN** an extra in-use-turn return is dropped because the semaphore is already full
- **THEN** the client emits `simpleredis_over_free` at Error with attributes `over_frees`, `turns`, and `cap`

#### Scenario: Pool wait is Warn
- **WHEN** a waiter exceeds `PoolTimeout`
- **THEN** the client emits `simpleredis_pool_exhausted` at Warn with attributes `pool_size`, `wait`, and `host`

#### Scenario: Short bulk is Warn
- **WHEN** the peer announced more bulk payload than it wrote
- **THEN** the client emits `simpleredis_short_bulk` at Warn with attributes `announced`, `read`, and `host`

#### Scenario: Malformed RESP is Warn
- **WHEN** the peer sends malformed or unsupported RESP (`redis:issue?` or `redis:unsupported-reply`)
- **THEN** the client emits `simpleredis_bad_reply` at Warn with attributes `error` and `host`

#### Scenario: Non-auth handshake failure is Warn
- **WHEN** AUTH or SELECT fails for a reason that is not AUTH-class (`LOADING`, max clients)
- **THEN** the client emits `simpleredis_handshake_failed` at Warn with attributes `error` and `host`

#### Scenario: Debug events fire on their paths
- **WHEN** a logger enabled at Debug is set on `Config` at `New`
- **AND** the client dials, sweeps idle, retries, times out, is canceled, closes a socket for cancel or idle cap, resolves MSetEX native vs Lua, reloads a script after NOSCRIPT, is constructed, or is closed
- **THEN** it emits the matching `simpleredis_dial`, `simpleredis_idle_swept`, `simpleredis_retry`, `simpleredis_timeout`, `simpleredis_canceled`, `simpleredis_socket_closed`, `simpleredis_capability`, `simpleredis_noscript`, `simpleredis_open`, or `simpleredis_close` at Debug

### Requirement: Logs never include secrets or Redis keys
No slog line at any level SHALL include `Config.Pass`, a Redis key name, or a Redis value. Attribute `host` SHALL be the server address `New` froze from `Config.Host`. `simpleredis_open` SHALL log frozen knobs and MUST NOT log `Pass`.

#### Scenario: Distinctive password and key never appear
- **WHEN** commands run with a distinctive password and a distinctive key name
- **AND** every slog line is captured at the most verbose level
- **THEN** neither string appears in the captured output

### Requirement: Debug paths allocate nothing when logging is off
A nil logger or a logger whose Debug level is disabled MUST NOT allocate slog attributes on per-command Debug events. The session MUST NOT install a discard handler to achieve that. A successful Get on a reused socket MUST NOT construct Debug attributes.

#### Scenario: Nil logger Get does not allocate Debug attrs
- **WHEN** `Config.Logger` is nil
- **AND** a Get runs on a reused socket
- **THEN** that Get does not allocate slog attribute slices for Debug events

#### Scenario: Debug-disabled logger Get does not allocate Debug attrs
- **WHEN** `Config.Logger` is non-nil and Debug is disabled
- **AND** a Get would otherwise be able to emit a Debug event
- **THEN** that path does not allocate slog attribute slices for Debug events

### Requirement: Interpreted logging works
Interpreted code SHALL construct a client with a `*slog.Logger` on `Config` and observe at least one exported `simpleredis_` event. Those tests MUST use GOPATH with stdlib symbols only. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi emits an open event
- **WHEN** interpreted code calls `New` with a capturing logger
- **THEN** the logger records `simpleredis_open`
