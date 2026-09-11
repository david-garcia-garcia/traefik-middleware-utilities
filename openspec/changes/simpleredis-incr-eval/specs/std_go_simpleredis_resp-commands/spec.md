## ADDED Requirements

### Requirement: Incr and IncrBy return the integer after increment
`Incr(name)` SHALL send Redis `INCR` for that key. `IncrBy(name, delta)` SHALL send Redis `INCRBY` with that key and the decimal delta, including when `delta` is `0`. Both SHALL return the integer value after the increment. A missing key SHALL NOT return `redis:miss`; the first increment SHALL behave as if the key started at `0`. A non-integer stored value SHALL return the server `-` error text (AUTH-class prefixes still map to `redis:noauth`). A `:` payload that is not a signed integer SHALL return `redis:issue?`.

#### Scenario: Incr missing then present
- **WHEN** Incr is called for a missing key
- **THEN** Incr returns `1`
- **WHEN** Incr is called again for that key
- **THEN** Incr returns `2`

#### Scenario: IncrBy missing
- **WHEN** IncrBy is called for a missing key with delta `5`
- **THEN** IncrBy returns `5`

#### Scenario: Incr of a non-integer value
- **WHEN** the key holds a non-integer string
- **AND** Incr is called
- **THEN** the caller receives that Redis error text
- **AND** the error is not `redis:issue?` unless the integer parse of a `:` reply failed

### Requirement: Expire and ExpireAt treat 0 and 1 as success
`Expire(name, seconds)` SHALL send Redis `EXPIRE` with that key and the decimal seconds the caller passed. `ExpireAt(name, unixSeconds)` SHALL send Redis `EXPIREAT` with that key and the decimal Unix timestamp the caller passed. An integer reply `0` or `1` SHALL return no error. The client MUST NOT treat `0` as `redis:miss`. The client MUST NOT clamp negative or zero seconds.

#### Scenario: Expire argv
- **WHEN** Expire is called with a key and a seconds value
- **THEN** the command sent is `EXPIRE`, that key, and those seconds as decimal digits
- **AND** an integer `0` or `1` reply returns no error

#### Scenario: ExpireAt argv
- **WHEN** ExpireAt is called with a key and a Unix timestamp
- **THEN** the command sent is `EXPIREAT`, that key, and that timestamp as decimal digits
- **AND** an integer `0` or `1` reply returns no error

### Requirement: Eval sends EVAL with numkeys equal to the key count
`Eval(script, keys, args)` SHALL send Redis `EVAL`, the script body, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. The return SHALL be the same `[][]byte` shape as other commands: a `:` integer is one element of decimal digits; a bulk is one element; a Lua or server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). EVALSHA and SCRIPT LOAD MUST NOT be added.

#### Scenario: Eval argv for one key
- **WHEN** Eval is called with a script, one key, and two args
- **THEN** the command sent is `EVAL`, that script, `1`, that key, then those args

#### Scenario: Eval integer reply
- **WHEN** Redis replies to EVAL with a `:` integer
- **THEN** Eval returns one `[][]byte` element whose bytes are that decimal payload

#### Scenario: Eval empty keys
- **WHEN** Eval is called with a script, no keys, and no args
- **THEN** the command sent includes `numkeys` `0`

### Requirement: Array replies accept bulk integer and status elements
An RESP array (`*`) SHALL accept each element whose head is `$` (bulk, including null bulk as a nil slot), `:` (integer payload bytes), or `+` (status payload bytes). If an element head is `*` or `-`, the client SHALL return `redis:issue?`. Nested arrays are out of scope. MGET callers MUST still observe only bulk slots from Redis MGET.

#### Scenario: Mixed array elements
- **WHEN** Redis replies with an array that contains a bulk, an integer, and a status
- **THEN** the result has three slots with those payloads
- **WHEN** an array element is a nested array
- **THEN** the client returns `redis:issue?`

## MODIFIED Requirements

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `Init`, `Get`, `Set`, `Del`, `Incr`, and `Eval` against a compiled fake TCP Redis. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi Init Get Set Del
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it Sets a key and Gets that key
- **AND** it Dels that key
- **THEN** Get returns the bytes that were Set
- **AND** a later Get of that key is a miss or the Del returned no error

#### Scenario: Yaegi Incr and Eval
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it calls Incr on a missing key
- **AND** it calls Eval with a script that returns an integer
- **THEN** Incr returns `1`
- **AND** Eval returns one element whose bytes are that integer

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new).

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`
