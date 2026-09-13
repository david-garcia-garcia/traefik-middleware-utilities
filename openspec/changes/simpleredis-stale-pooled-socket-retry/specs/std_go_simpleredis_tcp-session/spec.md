## ADDED Requirements

### Requirement: Peer-closed idle vintage is recovered sequentially
When every unused pooled TCP connection is closed by the peer at once (restart, failover, or kill of every accepted socket) while those sockets are still younger than `IdleTimeout`, sequential commands SHALL succeed while the peer is accepting. An I/O failure on a socket taken from idle MUST map to `redis:unreachable` and MUST NOT consume `MaxRetries`. The next attempt of that command SHALL be a new dial, not another unused pooled socket. That unused-socket force-dial SHALL happen at most once per command, including when `MaxRetries` is `-1`. A later sequential command MAY still take one leftover dead unused socket and then force-dial. A force-dial that itself fails with `redis:unreachable` SHALL consume `MaxRetries` as dest already does. Timeouts, handshake AUTH or SELECT failures, pool wait, and a closed client MUST NOT gain a free extra send. The in-use-turn channel SHALL stay full at rest with `OverFrees()` equal to 0. Compiled proof MUST warm unused sockets with simultaneous in-flight commands, close every accepted socket from the server, then issue sequential commands; it MUST NOT close the client-side file descriptors.

#### Scenario: Sequential Gets succeed after every idle socket is dropped
- **WHEN** `PoolSize` idle sockets have been warmed with simultaneous in-flight Gets
- **AND** the fake closes every accepted socket while remaining up and accepting
- **AND** sequential Gets are issued one after another
- **THEN** those Gets succeed
- **AND** none return `redis:unreachable` while the peer is accepting

#### Scenario: Default pool sequential recovery
- **WHEN** `PoolSize` is 8 and `MaxRetries` is the zero-Config default
- **AND** every idle socket is closed from the server
- **THEN** the next sequential Get succeeds

#### Scenario: MaxRetries off still force-dials once after unused-socket EOF
- **WHEN** `MaxRetries` is `-1`
- **AND** `PoolSize` idle sockets have been warmed and then closed from the server
- **AND** a sequential Get is issued
- **THEN** that Get succeeds on a new dial

#### Scenario: Turns stay balanced after vintage recovery
- **WHEN** sequential Gets have recovered after every idle socket was dropped
- **THEN** the in-use-turn channel is full
- **AND** `OverFrees()` is 0
