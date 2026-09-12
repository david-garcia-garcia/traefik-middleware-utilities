## MODIFIED Requirements

### Requirement: Traefik probe maps each SimpleRedis verb to an HTTP path
The nested SimpleRedis Traefik plugin SHALL dispatch on the last path segment after the engine mount. Exact `/redis` and `/dragonfly` SHALL run Set then Get of a unique per-request token (not a shared constant such as `"ok"`) so compose health can wait on those URLs. Each public verb SHALL be one path (`/get`, `/mget`, `/set`, `/del`, `/incr`, `/incrby`, `/expire`, `/expireat`, `/eval`, `/msetex`, `/msetexat`) that runs only that client method. Query `key` (repeatable), `arg` (repeatable), `ex`, `at`, `delta`, and `digest` SHALL be the method arguments. Set, Eval, MSetEX, and MSetEXAt SHALL take the request body as the value or Lua script. Eval SHALL pass query `digest` through as the SimpleRedis digest and MUST NOT hash the body. Query `drop=1` SHALL use `DropHost`. Success SHALL be HTTP 200 with the Redis payload in the body (empty when the method returns no payload). A command error SHALL be HTTP 502 with `err.Error()` in the body. The plugin MUST NOT copy results into `X-SimpleRedis-*` headers and MUST NOT forward a successful verb to whoami.

Unknown remainder on those routers SHALL return 404. Compose SHALL keep `PathPrefix(`/redis`)` and `PathPrefix(`/dragonfly`)` (no new routers). Handshake routers (`/redis-wrong-password`, `/dragonfly-wrong-password`, `/redis-database-99`, `/dragonfly-database-99`) SHALL keep a single Set+Get that 502s. Pester SHALL compose recover (Set then Get after `CLIENT KILL`), drop-relay (Get warmup then Incr and Eval with `drop=1`), and pool hold (concurrent Eval of a TIME-wait script). Reclaim `/a` `/b` stay up. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS and MUST NOT use `table.maxn`. SimpleRedis Pester SHALL be one file that reads `INTEGRATION_ENGINE` (`redis` or `dragonfly`) and MUST NOT duplicate Its or `-TestCases` per engine. CI SHALL prove EVALSHA + NOSCRIPT on Redis in `integration-redis` (`SCRIPT FLUSH` then `SCRIPT EXISTS` of the SHA-1 of the script body Pester sent is `0`, POST `/redis/eval` succeeds with body `3`, `EXISTS` is `1`, POST `/redis/eval` again succeeds with body `3`) and the same sequence on Dragonfly in `integration-dragonfly` via `redis-cli -h dragonfly` and `/dragonfly/eval`. This change MUST NOT add Redis or Dragonfly compose services.

#### Scenario: Pester asserts GET after SET
- **WHEN** Pester POSTs `/redis/set` then GETs `/redis/get` for the same key
- **THEN** the GET body is the value that SET wrote
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** CI `integration-redis` runs `./Test-Integration.ps1 -Suite simpleredis -Engine redis`
- **AND** CI `integration-dragonfly` runs the same SimpleRedis file with `-Engine dragonfly`
- **THEN** each response status and body show that verb succeeded on that job’s engine
- **AND** after `/msetex`, Eval of TTL is a positive decimal
- **AND** neither engine’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get and MGet own-value on Redis and Dragonfly
- **WHEN** two health requests are made on `/$Engine` for that job’s `INTEGRATION_ENGINE`
- **THEN** each response body is that request’s unique token
- **AND** two MGet requests on that engine have slot 0 equal to the value Pester Set
- **AND** the two health requests have distinct tokens
- **AND** those tests do not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get-miss on Redis and Dragonfly
- **WHEN** Pester GETs `/redis/get` and `/dragonfly/get` for a missing key
- **THEN** each response is HTTP 502 whose body is `redis:miss`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester proves EVALSHA miss then hit on Redis and Dragonfly
- **WHEN** Pester runs `SCRIPT FLUSH` then `SCRIPT EXISTS` of the script digest on Redis
- **THEN** EXISTS is `0`
- **WHEN** POST `/redis/eval` is made with that Lua body
- **THEN** the body is `3`
- **AND** `SCRIPT EXISTS` of that digest is `1`
- **WHEN** POST `/redis/eval` is made again with a new key
- **THEN** the body is `3`
- **WHEN** the same flush, EXISTS, POST, EXISTS, POST sequence runs against Dragonfly via `redis-cli -h dragonfly` and `/dragonfly/eval`
- **THEN** the same miss (`0`), POST success, hit (`1`), POST success holds
- **AND** neither Describe stops `whoami-a` or `whoami-b`

#### Scenario: Get MGet Incr Eval stay correct after ReadSlice decode
- **WHEN** compose is up with Redis and Dragonfly
- **AND** Pester hits `/$Engine/get`, `/$Engine/mget`, `/$Engine/incr`, `/$Engine/eval`, and `/$Engine/msetex` after `readLine` uses `ReadSlice`
- **THEN** those cases still show success on that job’s engine
- **AND** Eval remains Lua 5.1-safe with keys in KEYS

### Requirement: CI allocation guards fail on over-budget encode and decode
The compiled `go test` suite for SimpleRedis SHALL fail when client-side encode or decode of GET, EVAL, a bulk reply, a 10-slot array, an integer reply, or a 100 KB bulk exceeds the Go 1.21 `allocs/op` or `B/op` ceiling recorded in that test. Those guards SHALL run as compiled tests that measure `AllocsPerOp` and `AllocedBytesPerOp` without requiring `go test -bench`. They MUST NOT assert wall-clock `ns/op`. They MUST NOT dial live Redis or Dragonfly. Compose Redis (`redis:7-alpine`) and Dragonfly (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`) plus Pester HTTP verb paths under `/redis/` and `/dragonfly/` SHALL keep proving Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt on both engines. Eval SHALL stay a Lua 5.1-safe script that lists its key in KEYS. CI MUST NOT drop or skip either engine’s tests.

#### Scenario: Over-budget allocs fail without bench flag
- **WHEN** `go test ./simpleredis/...` runs without `-bench`
- **AND** a client-side encode or decode loop reports `AllocsPerOp` or `AllocedBytesPerOp` above the Go 1.21 ceiling recorded in that test
- **THEN** that test fails

#### Scenario: 100 KB bulk decode is guarded
- **WHEN** the decode guard runs a canned `$102400` bulk GET of `100*1024` bytes
- **THEN** `AllocsPerOp` and `AllocedBytesPerOp` are compared to the Go 1.21 ceiling
- **AND** the fixture is not a live Redis or Dragonfly round-trip

#### Scenario: Live verb coverage stays on Redis and Dragonfly
- **WHEN** CI `integration-redis` and `integration-dragonfly` run
- **THEN** Pester still asserts Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt on `/$Engine/<verb>`
- **AND** Eval uses a Lua 5.1-safe script with KEYS declared
- **AND** neither engine’s job is skipped

### Requirement: Live Redis and Dragonfly prove Lua MSetEX TTL landed
Compiled tests SHALL run `MSetEX` against each of Redis 7 and Dragonfly whose live address is set (`SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`). Those tests MUST skip under `-short` or when both addresses are unset. When exactly one address is set they MUST run that engine and MUST NOT fail for the missing engine. After `MSetEX`, `Get` SHALL return the written bytes and `Eval` of `TTL` on a declared KEYS key SHALL return a positive integer. `MSetEXAt` with a future Unix time SHALL then `Get` the written bytes and a positive TTL. `MSetEXAt` with a past timestamp SHALL then `Get` as a miss. CI `e2e-redis` MUST set `SIMPLEREDIS_LIVE_REDIS` to Redis 7 `:6379` and MUST NOT set `SIMPLEREDIS_LIVE_DRAGONFLY`. CI `e2e-dragonfly` MUST set `SIMPLEREDIS_LIVE_DRAGONFLY` to Dragonfly `:6380` and MUST NOT set `SIMPLEREDIS_LIVE_REDIS`. The unit `test` job MUST NOT set those vars. Pester path cases are not a substitute for this compiled live file.

#### Scenario: Live Redis TTL landed
- **WHEN** `SIMPLEREDIS_LIVE_REDIS` is set and tests are not `-short`
- **AND** MSetEX writes a key with a positive duration
- **THEN** Get returns the written bytes
- **AND** Eval of TTL for that key in KEYS returns a positive integer

#### Scenario: Live Dragonfly TTL landed
- **WHEN** `SIMPLEREDIS_LIVE_DRAGONFLY` is set and tests are not `-short`
- **AND** MSetEX writes a key with a positive duration
- **THEN** Get returns the written bytes
- **AND** Eval of TTL for that key in KEYS returns a positive integer

#### Scenario: Live future EXAT TTL landed
- **WHEN** a live address is set
- **AND** MSetEXAt is called with a Unix timestamp in the future
- **THEN** Get returns the written bytes
- **AND** Eval of TTL for that key in KEYS returns a positive integer

#### Scenario: Live past EXAT is a miss
- **WHEN** a live address is set
- **AND** MSetEXAt is called with a Unix timestamp in the past
- **THEN** a later Get of that key is `redis:miss`
