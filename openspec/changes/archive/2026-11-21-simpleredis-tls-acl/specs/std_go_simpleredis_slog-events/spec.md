## ADDED Requirements

### Requirement: A TLS failure is Error at the site that saw it
A `redis:tls` failure SHALL be logged at Error as `<site>: redis:tls` by the function that saw it (`dial` for the handshake, `do` for a read or write), with a `tls_error` attribute holding the `crypto/tls` error text. When that `do` line came from AUTH or SELECT, `dial` SHALL add `dial: redis:tls` at Warn with `verb`. The caller's error text SHALL stay exactly `redis:tls`.

#### Scenario: Handshake rejection is logged by dial
- **WHEN** the server certificate fails verification during the dial
- **THEN** the line is `dial: redis:tls` at Error
- **AND** it carries `tls_error` with the verification error text
- **AND** the caller's error text is exactly `redis:tls`

#### Scenario: First-read alert is logged by do
- **WHEN** the server refuses a missing client certificate on the first TLS 1.3 read of a command
- **THEN** the line is `do: redis:tls` at Error with `verb` naming that command and `tls_error` holding the alert text
- **WHEN** that first read was the AUTH reply
- **THEN** `do: redis:tls` at Error carries `verb` `AUTH`
- **AND** `dial` adds `dial: redis:tls` at Warn with `verb` `AUTH`

#### Scenario: The user name and password stay off TLS lines
- **WHEN** a TLS failure is logged for a client with a distinctive `User` and `Pass`
- **THEN** neither string appears in the captured output

## MODIFIED Requirements

### Requirement: A peer reply is not published
A log line MUST NOT contain any byte of a Redis error reply. When the failure is not a sentinel this package owns (`redis:unreachable`, `redis:timeout`, `redis:noauth`, `redis:tls`), the message SHALL be `<site>: redis error`. The session MUST NOT parse, trim, or republish peer text. The caller SHALL still receive the peer's full text. The `tls_error` attribute on a `redis:tls` line is the `crypto/tls` error text, not a Redis reply, and is not peer text in this sense.

Two replies are why: Redis 7.4 answers `AUTH` against a nopass default user with `ERR AUTH <password> called without any password configured for the default user`, which is not an AUTH-class prefix and so is never mapped to `redis:noauth`; and an unknown verb is refused with the command's own arguments quoted back (`ERR unknown command 'MSETEX', with args beginning with: '2', '<key>', '<value>'`), which is Redis key names and values.

#### Scenario: An AUTH reply that echoes the password stays off the line
- **WHEN** the peer answers AUTH with `ERR AUTH <password> called without any password configured for the default user`
- **THEN** the handshake line is `dial: redis error` at Warn with `verb` `AUTH`
- **AND** the captured output does not contain the password
- **AND** the caller's returned error still carries the peer's full text

#### Scenario: An unknown-command reply does not publish its arguments
- **WHEN** the peer refuses a verb with `ERR unknown command '<verb>', with args beginning with: '<key>'`
- **THEN** the line is `do: redis error` at Debug
- **AND** the captured output does not contain the key name

#### Scenario: A server state reply is not published either
- **WHEN** AUTH is answered with `LOADING Redis is loading the dataset in memory`
- **THEN** the lines are `dial: redis error` at Warn and `do: redis error` at Debug
- **AND** the captured output does not contain the rest of that reply

#### Scenario: A TLS failure keeps its own sentinel on the line
- **WHEN** a command fails with `redis:tls`
- **THEN** the site line is `<site>: redis:tls`, not `<site>: redis error`
