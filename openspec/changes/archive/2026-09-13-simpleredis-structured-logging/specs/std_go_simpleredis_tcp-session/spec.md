## MODIFIED Requirements

### Requirement: New records settings and does not dial
`New(Config)` SHALL copy `Config` onto a new client (host, password, database, pool knobs, I/O knobs, retry sentinels, and `Logger`) and SHALL create the in-use-turn channel sized to `PoolSize` (const default 8 when `PoolSize` is 0). `New` MUST NOT open a TCP connection. `New` MUST NOT return an error. After `New`, writes to the caller's `Config` or to the client MUST NOT change the live cap or the copied knobs. `SimpleRedis` MUST NOT export writable pool, timeout, retry, or logger fields.

Zero `Config` pool and timeout knobs SHALL mean the package defaults: `PoolSize` 8, `MaxIdleConns` 8, `PoolTimeout` 200 milliseconds, `IdleTimeout` 30 seconds, `DialTimeout` 200 milliseconds, `IOTimeout` 100 milliseconds. Retry fields on `Config`: `0` at `New` means 1 extra retry; `-1` means off (one send). An explicit `MaxRetries` of 3 means 3 extra retries. `MinRetryBackoff` / `MaxRetryBackoff` keep go-redis sentinels: `0` means 8ms / 512ms; `-1` means off.

A nil `Config.Logger` SHALL mean the client emits no slog lines. `New` MUST NOT replace a nil logger with a discard handler or any other handler. `log/slog` is a Go standard-library package and SHALL be an allowed import of the session source.

#### Scenario: New does not open a socket
- **WHEN** `New` is called with a host that refuses connections
- **THEN** `New` returns a client without error
- **AND** no TCP connection is opened

#### Scenario: Live cap is frozen at New
- **WHEN** `Config.PoolSize` is 1 at `New`
- **AND** `PoolSize` is written to 16 on that Config and on the client after `New`
- **AND** one command holds the only live turn
- **AND** another command waits past `PoolTimeout`
- **THEN** that waiter returns `redis:unreachable`
- **AND** the fake observes at most one TCP connection

#### Scenario: Nil logger is silent and New still succeeds
- **WHEN** `New` is called with a zero `Config` except `Host`
- **THEN** `New` returns a client without error
- **AND** later commands on that client MUST NOT panic for want of a logger
