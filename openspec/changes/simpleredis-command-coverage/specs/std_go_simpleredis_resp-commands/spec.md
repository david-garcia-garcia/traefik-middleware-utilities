## MODIFIED Requirements

### Requirement: Traefik request SET plus GET sets a response header
The nested SimpleRedis Traefik plugin SHALL dispatch on the last path segment after the engine mount. Exact `/redis` and `/dragonfly` SHALL run Set then Get of a unique per-request token (not a shared constant such as `"ok"`) so compose health can wait on those URLs. Each public-verb case SHALL run only that verb (plus the writes that case needs) against that backend and set one response result Pester can assert:

- `/get` — Set then Get; header value is the unique token
- `/mget` — Set then MGet of that key and a missing name; MGet slot 0 equals the token
- `/del` — Set then Del
- `/incr` — Incr of a missing key returns `1`
- `/incrby` — IncrBy of a missing key by `5` returns `5`
- `/expire` — Set then Expire
- `/expireat` — Set then ExpireAt
- `/eval` — Eval of the Kong KEYS snippet (INCRBY plus EXPIREAT when the key is new); result `3`; header `X-SimpleRedis-EvalDigest` is the SHA-1 hex of that snippet const
- `/get-miss` — Get of a missing key; result `redis:miss`
- `/msetex` — MSetEX then Eval TTL on that key in KEYS; TTL is a positive decimal
- `/msetexat` — MSetEXAt with a future Unix time then Get of that key

Unknown remainder on those routers SHALL return 404. Compose SHALL keep `PathPrefix(`/redis`)` and `PathPrefix(`/dragonfly`)` (no new routers). Handshake routers (`/redis-wrong-password`, `/dragonfly-wrong-password`, `/redis-database-99`, `/dragonfly-database-99`) SHALL keep a single Set+Get that 502s. Recover SHALL be `/redis/recover` and `/dragonfly/recover` (Set+Get only). Drop-relay SHALL be `/redis/drop` and `/dragonfly/drop` (Incr+Eval only). Hold SHALL be `/redis/hold` and `/dragonfly/hold`. Reclaim `/a` `/b` stay up. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS and MUST NOT use `table.maxn`. Pester SHALL prove EVALSHA + NOSCRIPT fallback live on both engines: `SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest is `0`, GET `/redis/eval` succeeds (`Eval` `3`), `EXISTS` is `1`, GET `/redis/eval` again succeeds. Same sequence against Dragonfly via `redis-cli -h dragonfly` and `/dragonfly/eval`. This change MUST NOT add Redis or Dragonfly compose services.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on `/redis/get`
- **THEN** the response includes a header whose value is the unique token GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** Pester requests `/redis/get`, `/redis/mget`, `/redis/del`, `/redis/incr`, `/redis/incrby`, `/redis/expire`, `/redis/expireat`, `/redis/eval`, `/redis/get-miss`, `/redis/msetex`, and `/redis/msetexat`
- **AND** the same cases under `/dragonfly/`
- **THEN** each response shows that case succeeded
- **AND** `/msetex` TTL is a positive decimal
- **AND** neither engine’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get and MGet own-value on Redis and Dragonfly
- **WHEN** two requests are made on `/redis/get`
- **AND** two requests are made on `/dragonfly/get`
- **THEN** each response’s Get header equals the unique token that request Set
- **AND** two requests on `/redis/mget` (and `/dragonfly/mget`) each have MGet slot 0 equal to that request’s Set token
- **AND** the two Get requests on the same engine have distinct tokens
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get-miss on Redis and Dragonfly
- **WHEN** a request is made on `/redis/get-miss` and on `/dragonfly/get-miss`
- **THEN** each response’s result is `redis:miss`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester proves EVALSHA miss then hit on Redis and Dragonfly
- **WHEN** Pester runs `SCRIPT FLUSH` then `SCRIPT EXISTS` of the probe digest on Redis
- **THEN** EXISTS is `0`
- **WHEN** GET `/redis/eval` is made
- **THEN** `X-SimpleRedis-Eval` is `3`
- **AND** `X-SimpleRedis-EvalDigest` is that SHA-1 hex
- **AND** `SCRIPT EXISTS` of that digest is `1`
- **WHEN** GET `/redis/eval` is made again
- **THEN** `X-SimpleRedis-Eval` is `3`
- **WHEN** the same flush, EXISTS, GET, EXISTS, GET sequence runs against Dragonfly via `redis-cli -h dragonfly` and `/dragonfly/eval`
- **THEN** the same miss (`0`), GET success, hit (`1`), GET success holds
- **AND** neither Describe stops `whoami-a` or `whoami-b`

#### Scenario: Get MGet Incr Eval stay correct after ReadSlice decode
- **WHEN** compose is up with Redis and Dragonfly
- **AND** Pester hits `/redis/get`, `/redis/mget`, `/redis/incr`, `/redis/eval`, and `/redis/msetex` (and the `/dragonfly/` twins) after `readLine` uses `ReadSlice`
- **THEN** those cases still show success on both engines
- **AND** Eval remains Lua 5.1-safe with keys in KEYS

### Requirement: CI allocation guards fail on over-budget encode and decode
The compiled `go test` suite for SimpleRedis SHALL fail when client-side encode or decode of GET, EVAL, a bulk reply, a 10-slot array, an integer reply, or a 100 KB bulk exceeds the Go 1.21 `allocs/op` or `B/op` ceiling recorded in that test. Those guards SHALL run as compiled tests that measure `AllocsPerOp` and `AllocedBytesPerOp` without requiring `go test -bench`. They MUST NOT assert wall-clock `ns/op`. They MUST NOT dial live Redis or Dragonfly. Compose Redis (`redis:7-alpine`) and Dragonfly (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`) plus Pester path cases under `/redis/` and `/dragonfly/` SHALL keep proving Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt on both engines. Eval SHALL stay a Lua 5.1-safe script that lists its key in KEYS. CI MUST NOT drop or skip either engine’s tests.

#### Scenario: Over-budget allocs fail without bench flag
- **WHEN** `go test ./simpleredis/...` runs without `-bench`
- **AND** a client-side encode or decode loop reports `AllocsPerOp` or `AllocedBytesPerOp` above the Go 1.21 ceiling recorded in that test
- **THEN** that test fails

#### Scenario: 100 KB bulk decode is guarded
- **WHEN** the decode guard runs a canned `$102400` bulk GET of `100*1024` bytes
- **THEN** `AllocsPerOp` and `AllocedBytesPerOp` are compared to the Go 1.21 ceiling
- **AND** the fixture is not a live Redis or Dragonfly round-trip

#### Scenario: Live verb coverage stays on Redis and Dragonfly
- **WHEN** CI integration runs
- **THEN** Pester still asserts Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt on `/redis/<case>` and `/dragonfly/<case>`
- **AND** Eval uses a Lua 5.1-safe script with KEYS declared
- **AND** neither engine’s tests are skipped

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
