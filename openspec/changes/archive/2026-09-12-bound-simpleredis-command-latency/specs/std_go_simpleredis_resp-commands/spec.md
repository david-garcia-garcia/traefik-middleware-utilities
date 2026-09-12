## ADDED Requirements

### Requirement: Public verbs have Context twins
Each public command (`Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`) SHALL have a matching `*Context` method whose first argument is a context. The unadorned method SHALL call the twin with `context.Background()`. Wire behavior of the unadorned methods SHALL stay the same as today except for the session overall deadline and zero-Config defaults specified on `std_go_simpleredis_tcp-session`.

#### Scenario: Get wraps GetContext
- **WHEN** Get is called
- **THEN** the session sends Redis GET for that key
- **AND** a bulk reply returns those bytes

#### Scenario: GetContext cancel does not send after cancel
- **WHEN** GetContext is called with a context that is already cancelled
- **THEN** the call returns that context's `Err()`
- **AND** the session MUST NOT send GET
