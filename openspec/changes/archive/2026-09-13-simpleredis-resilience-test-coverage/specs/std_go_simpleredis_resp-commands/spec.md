## ADDED Requirements

### Requirement: Keys and values with CRLF are data, not commands
The encoder SHALL write every argument as a length-prefixed RESP bulk string. A key and a value that each contain CRLF and an inline `PING` command payload MUST round-trip as stored data. A peer that parses strict RESP MUST treat those bytes as one argument and MUST NOT execute an injected command.

#### Scenario: CRLF and inline PING round-trip as data
- **WHEN** Set stores a key and a value that each contain CRLF and an inline PING payload
- **AND** Get reads that key from a strict-RESP fake
- **THEN** Get returns the same value bytes
- **AND** the fake did not treat the payload as a second command

### Requirement: Concurrent MSetEX unknown-command fallback stays balanced
Compiled unit tests SHALL run many overlapping MSetEX calls against a fake that rejects MSETEX. Those calls MUST succeed. After they finish, the in-use-turn channel SHALL be full and extra turn returns SHALL be zero.

#### Scenario: Sixteen overlapping MSetEX fallbacks
- **WHEN** many goroutines each call MSetEX repeatedly against a reject-MSETEX fake
- **THEN** every call returns no error
- **AND** the in-use-turn channel is full
- **AND** extra turn returns are zero

### Requirement: Interpreter tests observe error paths
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can run the dial-failure retry loop (including backoff), map an I/O timeout against a stalling peer, cancel a command in flight, wait on a full pool, and handle a truncated bulk, using GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. They MUST NOT panic in the interpreter. They MUST NOT assert `IsUnreachable` or `errors.Is` on AUTH or SELECT handshake failures. They MUST live in a test file that is not the existing happy-path Yaegi file.

#### Scenario: Interpreted dial failure retries without panic
- **WHEN** interpreted code Gets against a host that refuses the first dials
- **THEN** the interpreter does not panic
- **AND** the call returns an error the compiled tests already classify for that path

#### Scenario: Interpreted stall maps to timeout
- **WHEN** interpreted code Gets against a peer that accepts TCP and never replies
- **THEN** the interpreter does not panic
- **AND** the error text is `redis:timeout`

#### Scenario: Interpreted cancel, pool wait, and truncated bulk
- **WHEN** interpreted code cancels a command, waits on a full pool, or reads a truncated bulk
- **THEN** the interpreter does not panic
- **AND** each path returns the same classification the compiled suite already asserts
