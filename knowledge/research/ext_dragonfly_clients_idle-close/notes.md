# Dragonfly idle client close

How Dragonfly (dest pin `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, published 2026-09-03) closes a client TCP socket from the **server**. Dest compose and CI do not pass `--timeout`; default is 0.

## `--timeout` (idle close)

`--timeout`: close the connection after it is idle for N seconds; **0 disables**. Default **0**. ([Dragonfly server flags](https://www.dragonflydb.io/docs/managing-dragonfly/flags), [.sources/flags-timeout.md](.sources/flags-timeout.md))

`CONFIG SET` is fully supported on the compatibility matrix (verified against Dragonfly v1.40.0). This ticket did not measure whether `timeout` is a settable CONFIG key on v1.40.2. Do not rely on `CONFIG SET timeout` for CI: the GitHub `test` job shares `127.0.0.1:6380` with windowcounter and tokenbucket live tests. ([compatibility — CONFIG SET](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-client-kill.md](.sources/compatibility-client-kill.md))

If a test ever waits on idle `--timeout`, the wait MUST stay **below** SimpleRedis `idleTimeout` (30s) or `borrow` never sees peer EOF.

## `CLIENT KILL`

Dragonfly **partially** supports `CLIENT KILL`. Implemented filters: `ADDR ip:port`, `LADDR ip:port`, `ID client-id`, and the legacy single `ip:port` (same as `ADDR`). **Missing:** `MAXAGE`, `SKIPME`, `USER`. Compatibility table also lists `TYPE` as not in the missing list on the command page; the command page says `USER/TYPE/SKIPME` are not implemented — follow the command page (`source` for what this version documents). Do not use `SKIPME` or `TYPE`. Kill by `ID` or `ADDR`. Return is an integer count. ([Dragonfly CLIENT KILL](https://www.dragonflydb.io/docs/command-reference/server-management/client-kill), [.sources/client-kill.md](.sources/client-kill.md); [compatibility — CLIENT KILL](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-client-kill.md](.sources/compatibility-client-kill.md))

`CLIENT LIST` and `CLIENT ID` are fully supported.

## Test implication

Same as Redis: from a sidecar, `CLIENT KILL ADDR <pooled LocalAddr>` or `CLIENT KILL ID <id>`. Do not `CLIENT KILL TYPE NORMAL SKIPME YES` (Dragonfly has no `SKIPME`/`TYPE`). Pester can `docker compose exec redis redis-cli -h dragonfly` (redis:7-alpine has `redis-cli`; Dragonfly image is not assumed to).
