# Redis idle client close

How Redis 7 closes a client TCP socket from the **server** (idle `timeout`, or `CLIENT KILL`). Dest compose and CI `redis:7-alpine` ship `timeout 0` (never close idle clients).

## `timeout` (idle close)

Default is **0**: Redis does not close idle clients; the socket stays open. A positive `timeout` (seconds) closes a **normal** client that has been idle that long. Set it in `redis.conf` or `CONFIG SET timeout <seconds>`. It does **not** apply to Pub/Sub. The check is incremental, not a precise timer: a configured 10s may close at 12s under load. ([redis.io Client handling — Client Timeouts](https://redis.io/docs/latest/develop/reference/clients/), [.sources/clients-timeouts.md](.sources/clients-timeouts.md); [redis.conf `timeout 0`](https://github.com/redis/redis/blob/7.2/redis.conf), [.sources/redis-conf-timeout.md](.sources/redis-conf-timeout.md))

Do **not** `CONFIG SET timeout` on the shared CI Redis (`127.0.0.1:6379` also used by windowcounter and tokenbucket live tests in `go test ./...`). The value is process-wide and packages run in parallel.

If a test ever uses idle `timeout`, the wait after last use MUST stay **below** SimpleRedis `idleTimeout` (30s). Otherwise `borrow` drops the socket locally and never sees `io.EOF`.

## `CLIENT KILL`

Closes a given client connection. Legacy: `CLIENT KILL addr:port` matching `CLIENT LIST` `addr`. Filter form (Redis 2.8.12+): `CLIENT KILL ID <id>` / `ADDR ip:port` / `TYPE` / `SKIPME` / others; filters AND together; return is the killed count. `SKIPME YES` (default) leaves the calling connection. The victim does not notice until the **next** command (network error). Redis will not kill a client mid-command. ([redis.io CLIENT KILL](https://redis.io/docs/latest/commands/client-kill/), [.sources/client-kill.md](.sources/client-kill.md); [redis.io CLIENT LIST](https://redis.io/docs/latest/develop/reference/clients/), [.sources/clients-timeouts.md](.sources/clients-timeouts.md))

`CLIENT KILL` is `@admin` and `noscript` — not from Lua `EVAL`.

## Test implication

Prefer `CLIENT KILL ADDR <pooledConn.LocalAddr()>` (or `ID` from `CLIENT LIST`) from a **sidecar** connection. That closes only the socket under test, works with dest `timeout 0`, and does not wait on the imprecise idle timer.
