## Purpose

Defines the RESP commands a SimpleRedis client speaks after it holds a session: GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, and EVAL, plus the exported error strings callers match. Keys and values are opaque bytes. Interpreter tests prove Init/Get/Set/Del/Incr/Eval under Yaegi without starting Traefik.

## Requirements

### Requirement: Get returns the stored value or a miss
`Get(name)` SHALL send Redis `GET` for that key. A bulk reply SHALL return those bytes. A null bulk (`$-1`) SHALL return an error whose `Error()` text is `redis:miss`.

#### Scenario: Get hit and miss
- **WHEN** a key has been Set
- **AND** Get is called for that key
- **THEN** Get returns the bytes that were Set
- **WHEN** Get is called for a missing key
- **THEN** Get returns `redis:miss`

#### Scenario: Value with newlines survives
- **WHEN** Set stores a value that contains newline bytes
- **AND** Get is called for that key
- **THEN** Get returns those exact bytes

### Requirement: MGet returns aligned slots and skips empty names
`MGet(names)` SHALL send Redis `MGET` for those keys. Each null bulk slot SHALL be a nil slice in the result, aligned with the requested names. Empty or nil `names` SHALL return `nil, nil` and MUST NOT dial. A short array reply SHALL return an error whose `Error()` text is `redis:issue?`.

#### Scenario: MGet hits, misses, and empty
- **WHEN** MGet is called for a mix of present and missing keys
- **THEN** present keys return their bytes in the matching slots
- **AND** missing keys return nil in those slots
- **WHEN** MGet is called with no names
- **THEN** the result is `nil, nil`
- **AND** no TCP connection is opened

#### Scenario: MGet rejects a short reply
- **WHEN** Redis returns fewer array elements than requested names
- **THEN** MGet returns `redis:issue?`

### Requirement: Set writes with EX duration
`Set(name, data, duration)` SHALL send Redis `SET` with the key, the value, `EX`, and that duration as decimal seconds. A Redis error reply SHALL be returned to the caller (except AUTH-class prefixes, which map to `redis:noauth`).

#### Scenario: Set then Get round-trip
- **WHEN** Set writes a key with a positive duration
- **AND** Get is called for that key before expiry
- **THEN** Get returns the bytes that were Set

#### Scenario: Set returns a reply error
- **WHEN** Redis replies to SET with an error line that is not AUTH-class
- **THEN** Set returns that error text

### Requirement: Del removes a key
`Del(name)` SHALL send Redis `DEL` for that key. A status or integer reply SHALL be treated as success.

#### Scenario: Del succeeds
- **WHEN** Del is called against a Redis that replies `+OK` or an integer
- **THEN** Del returns no error

### Requirement: Exported error strings are stable
Callers SHALL match errors by `Error()` text. The session SHALL export these exact strings: `redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`. AUTH-class Redis error prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text.

#### Scenario: Rejected auth is redis:noauth
- **WHEN** Redis replies `-NOAUTH`
- **THEN** the command returns `redis:noauth`

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

### Requirement: Interpreter tests assert the unsafe conversion matrix
Tests that import Yaegi SHALL assert which `string`/`[]byte` conversions the interpreter accepts under stdlib-only symbols, stdlib plus unsafe symbols, and unrestricted. Those tests MAY register Yaegi unsafe symbols and MAY import `unsafe` in `_test.go` files. Existing Init/Get/Set/Del/Incr/Eval interpreter tests MUST still use GOPATH with stdlib symbols only and `useunsafe` false. Named copy-versus-unsafe benches SHALL exist so a human can reproduce the measured ns/op; they MUST NOT fail `go test` without `-bench`. Those tests MUST NOT start Traefik.

#### Scenario: Matrix cells match the measured table
- **WHEN** the unsafe-variant interpreter test runs under stdlib only, stdlib plus unsafe symbols, and unrestricted
- **THEN** go-redis v9 `unsafe.Slice` / `unsafe.String` is unsupported in every mode
- **AND** the legacy pointer-cast, struct-header, and `reflect.StringHeader` conversions are unsupported under stdlib only and supported when unsafe symbols are registered
- **AND** a cell mismatch fails the test

#### Scenario: Named copy versus unsafe benches exist
- **WHEN** a human runs the named compiled Eval-encode, parse-int, and interpreted convert benches
- **THEN** those benches measure copy versus unsafe conversions
- **AND** `go test` without `-bench` still passes

### Requirement: Compiled tests reject production unsafe
Compiled tests SHALL fail when a non-test file in the SimpleRedis session folder imports `unsafe` or `"C"`, or a non-stdlib dotted path. Compiled tests SHALL fail when the SimpleRedis probe plugin manifest or compose `simpleredisprobe` `useUnsafe` is true. Absent or false on the manifest SHALL pass. Those tests MUST NOT require an explicit `useUnsafe: false` on the manifest. Those tests MUST NOT scan the reclaim probe. Redis and Dragonfly Pester proofs of existing verbs MUST stay. Eval scripts that touch keys MUST list those keys (Dragonfly). Lua MUST stay 5.1-safe. EVALSHA MUST NOT be added.

#### Scenario: Session source import scan
- **WHEN** compiled tests list imports of non-test files in the SimpleRedis session folder
- **THEN** the test fails if any import path is `unsafe` or `"C"` or contains a dot
- **AND** `_test.go` files MAY import `unsafe`

#### Scenario: Probe useUnsafe scan
- **WHEN** compiled tests read the SimpleRedis probe Traefik manifest and the compose `simpleredisprobe` `useunsafe` setting
- **THEN** the test passes if the manifest field is absent or false and compose is false
- **AND** the test fails if either is true
- **AND** the reclaim probe is not scanned
