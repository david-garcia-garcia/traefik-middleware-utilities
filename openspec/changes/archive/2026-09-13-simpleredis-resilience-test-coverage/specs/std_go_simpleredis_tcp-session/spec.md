## ADDED Requirements

### Requirement: Chaos pool stays within live cap and does not mix keys
Compiled unit tests SHALL drive concurrent Gets against a peer that, per command, randomly replies honestly, delays a few milliseconds, closes with no reply, replies LOADING, or writes a short bulk then closes. After those callers stop, at rest the in-use-turn channel SHALL be full, extra turn returns SHALL be zero, and settled server-side open sockets SHALL be at most PoolSize. A non-error Get MUST return that key’s own value. Tests MUST NOT assert a peak live-socket count. Tests that sample idle length then in-use-turn length as a live-socket metric are invalid. `go test -short` MUST skip this stress.

#### Scenario: Chaos Get does not return another key
- **WHEN** many goroutines Get keys k0..k63 against that chaotic peer for a bounded duration
- **THEN** every Get that returns no error returns that key’s own value
- **AND** after quiescence the in-use-turn channel is full
- **AND** extra turn returns are zero
- **AND** settled server-side open sockets are at most PoolSize

#### Scenario: Short skips chaos stress
- **WHEN** `go test -short` runs the package
- **THEN** the chaos pool stress does not run

### Requirement: Close cycles do not leak goroutines or sockets
Compiled unit tests SHALL run many New / use / Close cycles and SHALL prove goroutine count is flat and server-side open sockets are zero. When N Gets are held, Close, then those Gets are released, idle SHALL be empty, server-side open sockets SHALL be zero, and the in-use-turn channel SHALL be full. `go test -short` MUST skip this stress.

#### Scenario: Repeated New use Close leaves no sockets
- **WHEN** a test constructs a client, issues commands, and Closes, many times
- **THEN** after the last Close, server-side open sockets are zero
- **AND** goroutine count is not higher than before the cycles

#### Scenario: Close during held Gets returns every turn
- **WHEN** N Gets are held at the peer, the client is Closed, and the holds are released
- **THEN** idle is empty
- **AND** server-side open sockets are zero
- **AND** the in-use-turn channel is full
