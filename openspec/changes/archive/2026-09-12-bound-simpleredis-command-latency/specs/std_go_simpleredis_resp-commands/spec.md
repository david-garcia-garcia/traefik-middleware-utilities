## ADDED Requirements

### Requirement: Public verbs take a context
Each public command (`Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`) SHALL take `context.Context` as its first argument. There SHALL NOT be a matching `*Context` twin or an unadorned method that wraps `context.Background()`. A caller with no deadline SHALL pass `context.Background()` at the call site. Wire behavior SHALL include the session overall deadline and zero-Config defaults specified on `std_go_simpleredis_tcp-session`.

#### Scenario: Get sends GET
- **WHEN** Get is called with a context and a key
- **THEN** the session sends Redis GET for that key
- **AND** a bulk reply returns those bytes

#### Scenario: Already-cancelled Get does not send
- **WHEN** Get is called with a context that is already cancelled
- **THEN** the call returns that context's `Err()`
- **AND** the session MUST NOT send GET
