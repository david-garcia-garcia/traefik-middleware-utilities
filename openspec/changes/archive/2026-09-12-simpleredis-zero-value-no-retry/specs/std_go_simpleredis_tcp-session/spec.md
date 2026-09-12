## ADDED Requirements

### Requirement: Client not from New fails immediately
A command on a `SimpleRedis` that did not come from `New` SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT be retried (MUST NOT sleep retry backoff). `New` remains the only constructor of the in-use-turn channel. `Close` on that client SHALL be idempotent and MUST NOT panic. Empty `MGet` and empty or mismatched `MSetEX` / `MSetEXAt` stay pre-dial validation and MUST NOT panic.

#### Scenario: Zero-value Get does not retry
- **WHEN** `Get` is called on `&SimpleRedis{}`
- **THEN** the command returns `redis:unreachable`
- **AND** it returns in under the default minimum retry backoff (8 milliseconds)

#### Scenario: Zero-value exported methods do not panic
- **WHEN** every exported command and `Close` is called on `&SimpleRedis{}` (non-empty `MGet` / `MSetEX` / `MSetEXAt` so they reach the session)
- **AND** `Close` is called a second time
- **THEN** no call panics
- **AND** each command that reaches the session returns `redis:unreachable`

#### Scenario: New remains the only in-use-turn constructor
- **WHEN** a client is `&SimpleRedis{}` and `New` has not run
- **THEN** that client has no in-use-turn channel
