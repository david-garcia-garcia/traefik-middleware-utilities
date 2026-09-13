## ADDED Requirements

### Requirement: Panic inside idle keep-or-close still unlocks the idle list
When the session decides whether a reusable socket returns to the idle pool, that decision SHALL run under the idle-list mutex, and that mutex SHALL be released even if the decision panics. The session MUST NOT hold that mutex across a socket close or an in-use-turn return. The in-use-turn return SHALL run after that decision returns, on every path, including when the socket is not reusable. The keep-or-close arithmetic SHALL still count this socket as in-use (its turn still held) while the decision runs. Extra turn returns SHALL stay 0. Session source MUST NOT add a production hook solely to panic inside that locked decision.

The close decision SHALL stay: close when the client is closed, or when the idle list is already at `MaxIdleConns` and live sockets are at `PoolSize`. Otherwise park. `lastUsed` SHALL still be stamped on the reusable path before the socket is parked, including when the later verdict is close.

#### Scenario: Reusable socket parks then the turn is returned
- **WHEN** a reusable socket is released while idle is under `MaxIdleConns` and live sockets are under `PoolSize`
- **THEN** that socket is in the idle list
- **AND** `lastUsed` was stamped before it was parked
- **AND** the in-use-turn channel is not short that turn
- **AND** extra turn returns stay 0

#### Scenario: Full idle at live cap closes then the turn is returned
- **WHEN** a reusable socket is released while idle is already at `MaxIdleConns` and live sockets are at `PoolSize`
- **THEN** that socket is closed, not parked
- **AND** the in-use-turn is returned after the idle-list mutex is released
- **AND** extra turn returns stay 0
