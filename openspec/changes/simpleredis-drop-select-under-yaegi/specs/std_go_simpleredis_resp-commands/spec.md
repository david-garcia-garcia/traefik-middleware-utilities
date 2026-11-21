## MODIFIED Requirements

### Requirement: Concurrent MSetEX unknown-command fallback stays balanced
Compiled unit tests SHALL run many overlapping MSetEX calls against a fake that rejects MSETEX. Those calls MUST succeed. After they finish, the in-use-turn semaphore SHALL be full (free turns equal its cap) and extra turn returns SHALL be zero.

#### Scenario: Sixteen overlapping MSetEX fallbacks
- **WHEN** many goroutines each call MSetEX repeatedly against a reject-MSETEX fake
- **THEN** every call returns no error
- **AND** the in-use-turn semaphore is full
- **AND** extra turn returns are zero
