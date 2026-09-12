## MODIFIED Requirements

### Requirement: Array replies accept bulk integer and status elements
An RESP array (`*`) SHALL accept each element whose head is `$` (bulk, including null bulk as a nil slot), `:` (integer payload bytes), or `+` (status payload bytes). If an element head is `*` or `-`, the client SHALL return `redis:issue?`. Nested arrays are out of scope. MGET callers MUST still observe only bulk slots from Redis MGET. A `:` or `+` slot SHALL be an independent copy of those payload bytes so a later read on the same connection cannot overwrite it.

#### Scenario: Mixed array elements
- **WHEN** Redis replies with an array that contains a bulk, an integer, and a status
- **THEN** the result has three slots with those payloads
- **WHEN** an array element is a nested array
- **THEN** the client returns `redis:issue?`

#### Scenario: Integer and status slots survive a later read
- **WHEN** an array reply stores a `:` payload and a `+` payload
- **AND** a later command is read on the same connection
- **THEN** those stored slots still equal the original payloads

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new). After RESP decode uses `ReadSlice`, Get, MGet, Incr, and Eval headers MUST still show those commands succeeded on both `/redis` and `/dragonfly`. This change MUST NOT add Redis or Dragonfly compose services.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Get MGet Incr Eval stay correct after ReadSlice decode
- **WHEN** compose is up with Redis and Dragonfly
- **AND** Pester hits `/redis` and `/dragonfly` after `readLine` uses `ReadSlice`
- **THEN** Get, MGet, Incr, and Eval headers still show success on both engines
- **AND** Eval remains Lua 5.1-safe with keys in KEYS
