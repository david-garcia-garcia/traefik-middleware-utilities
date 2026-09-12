## ADDED Requirements

### Requirement: Extra in-use turn return MUST NOT hang
When a caller returns an in-use-turn token while the in-use-turn channel is already full (`len` equals `cap` after `New`), that return SHALL complete without waiting. The send that would block MUST be dropped. `OverFrees()` SHALL increment by one for that drop. `OverFrees()` SHALL be readable next to `PoolSize()` and `MaxIdleConns()`. Balanced borrow and release SHALL leave the in-use-turn channel full (`len` equals `cap`) and `OverFrees()` equal to 0. The client MUST NOT panic on an extra return.

#### Scenario: Extra return on a fresh client returns promptly
- **WHEN** a client is created with `New` so the in-use-turn channel is full
- **AND** one extra in-use turn is returned without a matching take
- **THEN** that return completes without waiting
- **AND** `OverFrees()` is 1

#### Scenario: Hammered borrow and release stay balanced
- **WHEN** many goroutines hammer borrow and release through a healthy fake, a dead address, an AUTH-rejecting fake, and a starved pool with a short `PoolTimeout`
- **THEN** those goroutines all finish
- **AND** the in-use-turn channel `len` equals `cap`
- **AND** `OverFrees()` is 0
