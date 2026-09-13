## ADDED Requirements

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
