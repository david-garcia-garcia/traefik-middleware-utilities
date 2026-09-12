## ADDED Requirements

### Requirement: Panic after borrow still returns the in-use-turn
When a command has taken an in-use-turn and the command body panics before a normal return, the session SHALL still return that turn to the pool. The abandoned socket MUST NOT return to idle. The panic MUST propagate to the caller; the session MUST NOT recover it and MUST NOT map it to `redis:issue?`. After as many recovered panics as `PoolSize`, `len(inUseTurns)` SHALL equal `cap(inUseTurns)`, and a later `borrow` SHALL succeed without waiting `PoolTimeout`. Proof MUST be a same-package unit test against a canned peer whose reply header panics the parser (`*1000000000000000000\r\n` is sufficient today). That assertion MUST NOT depend on which panic source exists. Handshake AUTH/SELECT panics while `dial` still holds the turn are out of scope.

#### Scenario: Recovered panics refill the semaphore
- **WHEN** `PoolSize` is 2
- **AND** a canned peer replies `*1000000000000000000\r\n` on every command
- **AND** two command calls each panic inside the command body and are recovered by the test
- **THEN** `len(inUseTurns)` equals `cap(inUseTurns)`
- **AND** a later `borrow` succeeds
- **AND** that `borrow` does not wait `PoolTimeout`

#### Scenario: Panic is not recovered in the command loop
- **WHEN** a command panics inside the command body after taking an in-use-turn
- **THEN** that panic reaches the caller of the command
- **AND** the session does not convert it to `redis:issue?`
