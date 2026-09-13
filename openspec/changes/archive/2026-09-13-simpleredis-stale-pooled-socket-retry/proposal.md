## Why

After a Redis restart, failover, or `CLIENT KILL` of every idle socket, sequential SimpleRedis commands return `redis:unreachable` while the peer is healthy and accepting. Dest retry spends `MaxRetries+1` dead idle sockets per command and leaves the rest parked. Parallel traffic hides it; a Traefik rate-limiter after failover is usually sequential.

## What Changes

- After an I/O failure on a socket taken from idle, later attempts of that command skip idle and dial instead of popping the next corpse. `MaxRetries` still counts. No free extra send: a dead unused socket can fail after write succeeds, which is the same shape as a lost reply.
- `borrow`'s three-value signature stays. `takeIdleConn` is not rewritten (BUG-6 owns that path). Leftover idle corpses stay parked; later sequential commands each spend one then force-dial.
- Permanent untagged default-suite test: real TCP, in-process RESP fake, warm idle with simultaneous in-flight commands, drop every accepted socket, sequential Gets succeed. New fake/helper types use a bug-specific prefix.
- No config knob. No pool-generation / epoch. No idle-list wipe from the command path.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: when every idle socket is closed by the peer at once, sequential commands SHALL succeed on a new dial while the peer is accepting. An I/O failure on a reused idle socket is not evidence the peer is down and MUST NOT consume `MaxRetries`. Dest's one-socket peer-close scenarios stay.

## Impact

- `simpleredis/pool.go` (`borrow` wrapper + shared body). `simpleredis/commands_exec.go` (`exec` retry accounting).
- New untagged `simpleredis/*_test.go` with a bug-prefixed fake. Reuse `readCommand` / `pooledIdle` / `assertTurnsFullAndNoOverFrees`.
- Main spec `std_go_simpleredis_tcp-session` after archive.
- Usage `knowledge/devdocs/std_go_simpleredis.md` one-socket retry wording (devdocs-impact).
