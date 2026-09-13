## ADDED Requirements

### Requirement: Abandoned sockets after a recovered panic MUST be closed
When a command takes a socket and never returns it (a panic between take and return that Traefik recovers), the next waiter that restores leaked turns SHALL close that socket once its checkout is at least as old as the command budget `(maxRetries+1)*(DialTimeout+IOTimeout)` already used as the overall command deadline. Recovery MUST keep a pointer to each checked-out socket on the client until `release` or that close. Reclaim MUST run only on that pool-wait path. `New` MUST NOT start a goroutine to close abandoned sockets. A command still within that budget, including a socket between borrow and the command body and between the command body and return to idle, MUST NOT be closed. `AbandonedClosed()` SHALL increment by the number of sockets this path closes and SHALL be readable next to `LostTurns()`, `OverFrees()`, `PoolSize()`, and `MaxIdleConns()`. `LostTurns()` SHALL still count restored in-use-turn tokens. `exec` MUST NOT defer `release` as the recovery.

#### Scenario: Recovered panics eventually close abandoned sockets
- **WHEN** a client of `PoolSize` 2 has warmed one idle socket
- **AND** `PoolSize` callers each take a turn via `borrow` then panic with `recover()` in the caller and never `release`
- **AND** at least one command budget elapses
- **AND** a later Get runs
- **THEN** that Get succeeds
- **AND** the fake's still-open sockets settle at a bounded live count (not the accept count)
- **AND** `LostTurns()` is at least `PoolSize`

#### Scenario: A live in-flight command is not reclaimed
- **WHEN** one command is genuinely in flight on its socket
- **AND** another command waits one `PoolTimeout` and hits pool-wait recovery
- **THEN** the in-flight command completes with a correct reply
- **AND** the fake still has that command's socket open
- **AND** `AbandonedClosed()` does not include that socket

#### Scenario: A socket in the borrow-to-do or do-to-release gap is not reclaimed
- **WHEN** a caller has borrowed a socket and has not yet returned it to idle
- **AND** checkout age is below the command budget
- **AND** another command waits one `PoolTimeout` and hits pool-wait recovery
- **THEN** that borrowed socket stays open
- **AND** a later command on that socket still succeeds

#### Scenario: LostTurns and AbandonedClosed report honestly
- **WHEN** a client runs a sequential Get against a live fake
- **THEN** `LostTurns()` is 0
- **AND** `AbandonedClosed()` is 0
- **WHEN** `PoolSize` recovered panics then a Get runs before one command budget elapses
- **THEN** `LostTurns()` is at least `PoolSize`
- **AND** `AbandonedClosed()` is 0
- **WHEN** one command budget then elapses and a later Get runs
- **THEN** `AbandonedClosed()` is at least `PoolSize`
