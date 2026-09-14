## Purpose

Defines how a SimpleRedis client logs: every failure is reported by the function that saw it, so one error text no longer hides five different causes, and the few conditions that are not failures keep a named event. No log line may carry `Config.Pass`, a Redis key name, or a Redis value.

## Requirements

### Requirement: A failure is logged by the site that saw it
When the session decides a command failed, it SHALL emit one slog line whose message is `<site>: <cause>`, where `<site>` is the function that saw the failure (`borrowSocket`, `dial`, `do`, `exec`). The session MUST NOT classify a failure into a `simpleredis_*` message token (`simpleredis_canceled`, `simpleredis_timeout`, `simpleredis_noauth`, `simpleredis_pool_exhausted`, `simpleredis_handshake_failed`, `simpleredis_bad_reply`, `simpleredis_short_bulk`, `simpleredis_auth_leftover`, `simpleredis_noscript`). Site names live in one unexported constant block; the session MUST NOT export them.

The site helper SHALL log only. It MUST NOT construct or wrap an error, and the caller SHALL return the error it already had. `Error()` of every exported sentinel therefore stays exactly the text `std_go_simpleredis_tcp-session` pins (`redis:unreachable`, `redis:timeout`, `redis:noauth`, `redis:issue?`, `redis:miss`, `redis:unsupported-reply`), and the existing `err.Error()` prefix checks in `commands_eval.go`, `commands_msetex.go`, `commands_exec.go` and `resp.go` SHALL keep matching the raw error. The session MUST NOT add an unwrap helper for those checks. A site helper that returns a `fmt.Errorf("%s: %w", site, err)` is rejected: `RedisUnreachable` and its siblings are exported text that `windowcounter`, `tokenbucket`, `leakybucket` and `e2e/simpleredisprobe` compare against, so the prefix would break them and every later prefix check would silently misclassify.

Where two paths in one site return the same cause, the line SHALL carry a `reason` attribute that separates them (`closed`, `not_from_new`, `pool_wait`, `set_deadline`, `unread_before_write`, `write`, `read`). A `do` line SHALL carry `verb`, the Redis command in flight (`args[0]`, never a key or a value). The session MUST NOT emit an Info-level line.

#### Scenario: One unreachable text, told apart by its site
- **WHEN** a command fails because the TCP dial failed
- **THEN** the line is `dial: redis:unreachable` at Debug
- **AND** a command on a closed client is `borrowSocket: redis:unreachable` at Debug with `reason` `closed`
- **AND** both callers still return an error whose `Error()` text is exactly `redis:unreachable`

#### Scenario: AUTH rejection is Error at the site that read it
- **WHEN** AUTH is rejected with `WRONGPASS`, `NOAUTH`, `NOPERM`, or `ERR Client sent AUTH`
- **THEN** the line is `do: redis:noauth` at Error
- **AND** `dial` adds `dial: redis:noauth` at Warn with `verb` `AUTH`, because only `dial` knows which handshake step failed
- **AND** the caller's error text is exactly `redis:noauth`

#### Scenario: Pool wait is Warn at borrowSocket
- **WHEN** a waiter exceeds `PoolTimeout`
- **THEN** the line is `borrowSocket: redis:unreachable` at Warn with `reason` `pool_wait`

#### Scenario: Short bulk is Warn at do
- **WHEN** the peer announced more bulk payload than it wrote
- **THEN** the line is `do: redis:unreachable` at Warn with `reason` `read`

#### Scenario: Malformed RESP is Warn at do
- **WHEN** the peer sends malformed or unsupported RESP (`redis:issue?` or `redis:unsupported-reply`)
- **THEN** the line is `do: redis:issue?` or `do: redis:unsupported-reply` at Warn

#### Scenario: Unread bytes before a write are Warn at do
- **WHEN** unread bytes remain in the reader before the next command is written
- **THEN** the line is `do: redis:unreachable` at Warn with `reason` `unread_before_write`
- **AND** `verb` names the command that was refused, which is `AUTH` or `SELECT` for handshake leftover

#### Scenario: Cancel and the command budget are exec's, once
- **WHEN** the caller cancels, or a caller deadline fires sooner than the budget
- **THEN** the line is `exec: context canceled` or `exec: context deadline exceeded` at Debug
- **AND** when the library's own `CommandTimeout` expires the line is `exec: redis:timeout` at Debug
- **AND** `do` MUST NOT emit a `redis:timeout` line: `ioOrContext` hands the socket deadline up as a context deadline and `libraryTimeout` in `exec` is the single owner of library-budget versus caller-deadline, so a `do`-side emit would either never fire or duplicate that one

### Requirement: A peer reply is published as its error code only
A log line MUST NOT contain any byte of a Redis error reply after its leading error code. The one sink that formats a site line SHALL print the error's own text when the error is a sentinel this package owns or a context error, and otherwise SHALL print only the leading `A`-`Z` token of the peer's text (`ERR`, `LOADING`, `MISCONF`, `NOSCRIPT`, `WRONGTYPE`); a leading token that is not all `A`-`Z` SHALL not be published at all. The rule SHALL live at that sink, so no call site decides it and no site, level, or verb can bypass it. The caller SHALL still receive the peer's full text.

Two replies are why: Redis 7.4 answers `AUTH` against a nopass default user with `ERR AUTH <password> called without any password configured for the default user`, which is not an AUTH-class prefix and so is never mapped to `redis:noauth`; and an unknown verb is refused with the command's own arguments quoted back (`ERR unknown command 'MSETEX', with args beginning with: '2', '<key>', '<value>'`), which is Redis key names and values. Logging the peer's raw text at a site, on any path, is rejected.

#### Scenario: An AUTH reply that echoes the password is published as ERR
- **WHEN** the peer answers AUTH with `ERR AUTH <password> called without any password configured for the default user`
- **THEN** the handshake line is `dial: ERR` at Warn with `verb` `AUTH`
- **AND** the captured output does not contain the password
- **AND** the caller's returned error still carries the peer's full text

#### Scenario: An unknown-command reply does not publish its arguments
- **WHEN** the peer refuses a verb with `ERR unknown command '<verb>', with args beginning with: '<key>'`
- **THEN** the line is `do: ERR` at Debug
- **AND** the captured output does not contain the key name

#### Scenario: A server state code survives the trim
- **WHEN** AUTH is answered with `LOADING Redis is loading the dataset in memory`
- **THEN** the lines are `dial: LOADING` at Warn and `do: LOADING` at Debug
- **AND** the captured output does not contain the rest of that reply

### Requirement: Logs never include secrets or Redis keys
No slog line at any level SHALL include `Config.Pass`, a Redis key name, or a Redis value. Attribute `host` SHALL be the server address `New` froze from `Config.Host` when it is logged. A `verb` attribute SHALL be `args[0]` only, which is the Redis command name.

A wrong-password proof is not sufficient coverage on its own: `WRONGPASS` is AUTH-class, so it becomes the `redis:noauth` sentinel and never carries peer text. Coverage MUST also pin the nopass-default-user reply, which is not AUTH-class and is the reply that contains the credential.

#### Scenario: Distinctive password and key never appear
- **WHEN** commands run with a distinctive password and a distinctive key name
- **AND** every slog line is captured at the most verbose level
- **THEN** neither string appears in the captured output

### Requirement: A condition with no error keeps a named event
A condition that is not a failed command SHALL stay a named `simpleredis_*` event at the decision site with inline slog attributes, because a site line reports an error and these paths have none. The session MUST NOT invent an error value so that a non-failure can be logged as one. Those events are exactly `simpleredis_open`, `simpleredis_dial`, `simpleredis_idle_swept`, `simpleredis_retry`, `simpleredis_capability`, `simpleredis_over_free`, `simpleredis_socket_poisoned` and `simpleredis_panic`. The session MUST NOT export message-name constants and MUST NOT add a logging helper file.

#### Scenario: Non-failures still log
- **WHEN** a logger enabled at Debug is set on `Config` at `New`
- **THEN** construction emits `simpleredis_open` at Debug with `host`
- **AND** a successful dial emits `simpleredis_dial` at Debug, an idle sweep emits `simpleredis_idle_swept` at Debug, an extra attempt emits `simpleredis_retry` at Debug, and resolving native MSETEX versus the Lua fallback emits `simpleredis_capability` at Debug
- **AND** a dropped in-use-turn return emits `simpleredis_over_free` at Error, leftover after a complete reply emits `simpleredis_socket_poisoned` at Warn, and a recovered panic emits `simpleredis_panic` at Error before it is re-raised

#### Scenario: Panic is Error then re-raised
- **WHEN** `do` panics inside `runOnConn`
- **THEN** the client emits `simpleredis_panic` at Error
- **AND** the panic still reaches the caller
- **AND** the in-use turn is returned, the socket is closed, and `OverFrees()` is `0`

### Requirement: Dial names why it dialled
`simpleredis_dial` SHALL carry a `reason` attribute. `reason` SHALL be `idle_miss` when the borrow consulted the unused-socket list and had nothing young to reuse, and `skip_idle` when the borrow bypassed that list because this command already failed I/O on a socket it took from it. Operators diagnose a peer restart, a failover, or a `CLIENT KILL` of the accepted sockets from a burst of `skip_idle`, and pool pressure from a burst of `idle_miss`; a dial event that does not separate those two is not enough to tell them apart, and no site line reports a dial that worked. The session MUST NOT reuse one reason string for both cases.

#### Scenario: Bypassing a dropped unused list says skip_idle
- **WHEN** the unused list is warmed, every accepted socket at the peer is then dropped, and a command borrows one of those sockets and retries
- **THEN** a `simpleredis_dial` line carries `reason` `skip_idle`
- **AND** a `simpleredis_dial` line from warming the unused list carries `reason` `idle_miss`

### Requirement: Interpreted logging works
Interpreted code SHALL construct a client with a `*slog.Logger` on `Config`, observe `simpleredis_open`, and then fail one command so that a site line is emitted too. The site emit SHALL be exercised under the interpreter because it calls `Logger.Log` with a level argument, which is a different slog surface than the `Debug`, `Warn` and `Error` calls the named events use. A client that did not come from `New` has no logger, so the site helper SHALL check for a nil logger and MUST NOT panic. Those tests MUST use GOPATH with stdlib symbols only. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi emits an open event and a dial site line
- **WHEN** interpreted code calls `New` with a capturing logger and then `Get` against a closed port
- **THEN** the logger records `simpleredis_open`
- **AND** it records a message equal to `dial: redis:unreachable`
- **AND** the captured output does not contain the configured password
