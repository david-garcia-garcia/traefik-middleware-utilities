## MODIFIED Requirements

### Requirement: Live cap is proven on Redis and Dragonfly
The session SHALL keep at most `poolSize` live TCP connections (idle plus in use; const default 8) against a real Redis and a real Dragonfly. Fake-server tests MUST NOT be the only proof. Traefik local-plugin Pester on `/redis` and `/dragonfly` SHALL overlap requests long enough to contend for sockets, observe at most `poolSize` clients on that backend (default 8), and observe `redis:unreachable` when a waiter exceeds the pool wait. Compiled tests gated on live addresses SHALL prove pool-wait `redis:unreachable` on each engine whose address is set; they MAY set `Config.PoolSize` so a shared CI Redis is not left in Lua BUSY. Those compiled tests MUST skip under `-short` or when both `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY` are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. Any Lua used to hold a socket MUST be Lua 5.1-safe (no `table.maxn`) and MUST list touched keys in KEYS (zero keys when none are touched). Compose project `reclaim-e2e`, routes `/a` `/b`, and existing verb headers MUST keep their semantics.

#### Scenario: Extra waiter is redis:unreachable on the set engine
- **WHEN** `SIMPLEREDIS_LIVE_REDIS` is set and tests are not `-short`
- **AND** `poolSize` sockets are held against Redis
- **AND** another command cannot obtain a socket before the pool wait
- **THEN** that command fails with `redis:unreachable`
- **WHEN** `SIMPLEREDIS_LIVE_DRAGONFLY` is set and tests are not `-short`
- **THEN** the same waiter fails with `redis:unreachable` against Dragonfly

### Requirement: Peer-closed idle socket is retried
When a pooled idle TCP connection is closed by the Redis or Dragonfly peer while it is still younger than thirty seconds, the next command SHALL treat that failure as a dead connection (not a timeout) and SHALL retry on a new dial under the go-redis-shaped `MaxRetries` policy. An I/O end-of-file on that reused socket MUST map to an error whose `Error()` text is `redis:unreachable`. A timeout MUST NOT be retried. Closing the client-side file descriptor of a pooled socket is a distinct failure and MUST remain a separate proof; that path MUST NOT stand in for peer close. If the retry cannot obtain a connection, the command SHALL return `redis:unreachable`. The dead socket MUST NOT be returned to the idle pool.

Compiled tests MUST close the **accepted** socket from the server after the first reply and MUST NOT close the client. Live tests MUST close the pooled connection with `CLIENT KILL` by `ADDR` or `ID` (not `TYPE` or `SKIPME`) against each engine whose SimpleRedis live address is set, then the next command SHALL succeed on a new dial. Those live tests MUST skip under `-short` or when both SimpleRedis live addresses are unset, MUST run the set engine when exactly one address is set, and MUST run on CI `e2e-redis` and `e2e-dragonfly`. The nested Traefik plugin SHALL keep `simpleredis.New` in Traefik `New`. A recover request (`recover=1`) SHALL run Set and Get only, SHALL set `X-SimpleRedis-Recover: ok` when those succeed after recovery, and MUST NOT Eval. Default `/redis` and `/dragonfly` verb headers MUST stay. Existing Eval on the default path SHALL remain Lua 5.1-safe and SHALL list its keys in `KEYS`. Compose idle `timeout` SHALL stay 0. The SimpleRedis Pester Describe MUST NOT stop `whoami-a` or `whoami-b`.

#### Scenario: Live CLIENT KILL recovers on the set engine
- **WHEN** `SIMPLEREDIS_LIVE_REDIS` is set and tests are not `-short`
- **THEN** after `CLIENT KILL` of the pooled socket the next command succeeds on a new dial against Redis
- **WHEN** `SIMPLEREDIS_LIVE_DRAGONFLY` is set and tests are not `-short`
- **THEN** the same recovery succeeds against Dragonfly
