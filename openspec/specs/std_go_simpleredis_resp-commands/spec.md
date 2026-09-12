## Purpose

Defines the RESP commands a SimpleRedis client speaks after it holds a session: GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL, MSetEX, MSetEXAt, and ExecPipeline, plus the exported error strings callers match. Keys and values are opaque bytes. Interpreter tests prove New/Get/Set/Del/Incr/Eval/MSetEX/ExecPipeline under Yaegi without starting Traefik.

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
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `New`, `Get`, `Set`, `Del`, `Incr`, `Eval`, `MSetEX`, and `ExecPipeline` against a compiled fake TCP Redis, including the NOSCRIPT fallback path. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. Yaegi SHALL cover both MSetEX paths: a fake that implements MSETEX (native), and a fake that rejects MSETEX so the first call falls back to EVAL and a second call does not send `MSETEX`.

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

#### Scenario: Yaegi Eval NOSCRIPT fallback
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener whose first EVALSHA for that script is a miss
- **AND** it calls Eval
- **THEN** Eval returns the script result
- **AND** the caller does not see NOSCRIPT

#### Scenario: Yaegi ExecPipeline
- **WHEN** interpreted code builds a client with `New` to a compiled fake Redis listener
- **AND** it calls ExecPipeline with INCR then GET of that key
- **THEN** the batch error is nil
- **AND** the INCR slot value is `1`
- **AND** the GET slot carries those bytes

#### Scenario: Yaegi MSetEX native
- **WHEN** interpreted code constructs a client with New to a compiled fake that implements MSETEX
- **AND** it calls MSetEX with one name and value
- **THEN** MSetEX returns no error
- **AND** Get of that name returns the written bytes

#### Scenario: Yaegi MSetEX Lua fallback
- **WHEN** interpreted code constructs a client with New to a compiled fake that replies unknown-command to MSETEX
- **AND** it calls MSetEX twice
- **THEN** the first call returns no error
- **AND** the second call does not send `MSETEX`

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt against that backend, and set one response header per verb. Eval SHALL run twice on that request: `X-SimpleRedis-Eval` from the first result, `X-SimpleRedis-EvalAgain` from the second. The probe SHALL set `X-SimpleRedis-EvalDigest` to the SHA-1 hex of the Kong KEYS snippet const. After MSetEX it SHALL Eval `TTL` on that key listed in KEYS and set `X-SimpleRedis-MSetEX-TTL` to the positive TTL decimal. After those verbs, the same request SHALL call ExecPipeline with INCR, EXPIRE, GET of that incr key, and EVAL of a Lua 5.1-safe Kong INCRBY+EXPIREAT snippet that lists `KEYS[1]` on a distinct per-request eval key, and SHALL set `X-SimpleRedis-Pipeline` to `1:ok:1:3`. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Compose MUST NOT add a new route, engine, or `--pipeline_queue_limit` flag. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new) and MUST NOT use `table.maxn`. Pester SHALL prove EVALSHA + NOSCRIPT fallback live on both engines: `SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest is `0`, GET succeeds (`Eval` `3` and `EvalAgain` `3`), `EXISTS` is `1`, GET again succeeds. Same sequence against Dragonfly via `redis-cli -h dragonfly`. Existing verb headers stay. Live mixed-verb pipeline proof on Redis and Dragonfly is REQUIRED; a flush-counting fake MUST NOT substitute for it. Reclaim `/a` `/b` stay up. After RESP decode uses `ReadSlice`, Get, MGet, Incr, Eval, MSetEX, and Pipeline headers MUST still show those commands succeeded on both `/redis` and `/dragonfly`. This change MUST NOT add Redis or Dragonfly compose services.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEX-TTL that show those commands succeeded
- **AND** the MSetEX-TTL header is a positive decimal
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester proves EVALSHA miss then hit on Redis and Dragonfly
- **WHEN** Pester runs `SCRIPT FLUSH` then `SCRIPT EXISTS` of the probe digest on Redis
- **THEN** EXISTS is `0`
- **WHEN** GET `/redis` is made
- **THEN** `X-SimpleRedis-Eval` is `3`
- **AND** `X-SimpleRedis-EvalAgain` is `3`
- **AND** `X-SimpleRedis-EvalDigest` is that SHA-1 hex
- **AND** `SCRIPT EXISTS` of that digest is `1`
- **WHEN** GET `/redis` is made again
- **THEN** `X-SimpleRedis-Eval` is `3`
- **WHEN** the same flush, EXISTS, GET, EXISTS, GET sequence runs against Dragonfly via `redis-cli -h dragonfly` and `/dragonfly`
- **THEN** the same miss (`0`), GET success, hit (`1`), GET success holds
- **AND** neither Describe stops `whoami-a` or `whoami-b`

#### Scenario: Pester asserts the pipeline header on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes `X-SimpleRedis-Pipeline` with value `1:ok:1:3`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Get MGet Incr Eval stay correct after ReadSlice decode
- **WHEN** compose is up with Redis and Dragonfly
- **AND** Pester hits `/redis` and `/dragonfly` after `readLine` uses `ReadSlice`
- **THEN** Get, MGet, Incr, Eval, and MSetEX headers still show success on both engines
- **AND** Eval remains Lua 5.1-safe with keys in KEYS

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

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(script, keys, args)` SHALL keep the public signature `Eval(script string, keys []string, args []string) ([][]byte, error)`. Callers pass the script body; they MUST NOT pass a digest. `Eval` SHALL hash the script body on each call (SHA-1 lowercase hex; cheap; no map, no lock) and send Redis `EVALSHA`, that digest, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args. That EVAL is the only place the script body is sent to Redis/Dragonfly so the engine stores it. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The client MUST NOT keep a digest table or mutex for scripts. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

#### Scenario: Later Eval sends EVALSHA not the body
- **WHEN** Eval is called twice with the same script, one key, and two args against a fake that already has that digest
- **THEN** the second command sent is `EVALSHA`, the SHA-1 hex of that script, `1`, that key, then those args
- **AND** the second argv MUST NOT include the script body

#### Scenario: First EVALSHA miss falls back to EVAL
- **WHEN** the first `EVALSHA` for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends `EVAL`, that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: After EVAL the digest hits
- **WHEN** EVAL has loaded that digest on the fake
- **AND** Eval is called again with the same script
- **THEN** the command sent is `EVALSHA` with that digest
- **AND** Eval succeeds without a second EVAL

#### Scenario: Two scripts two digests
- **WHEN** Eval is called with script A then with a different script B
- **THEN** the `EVALSHA` argv digests differ

#### Scenario: Eval integer reply
- **WHEN** Redis replies to EVALSHA or EVAL with a `:` integer
- **THEN** Eval returns one `[][]byte` element whose bytes are that decimal payload

#### Scenario: Eval empty keys
- **WHEN** Eval is called with a script, no keys, and no args
- **THEN** the command sent includes `numkeys` `0`

### Requirement: MSetEX and MSetEXAt write many keys with one shared TTL
`MSetEX(names, values, seconds)` SHALL send native Redis `MSETEX` with decimal `numkeys` equal to `len(names)`, then each name/value pair in order, then `EX` and that duration as decimal seconds. `MSetEXAt(names, values, unixSeconds)` SHALL send the same argv with `EXAT` and that Unix timestamp. Both SHALL require `len(names) == len(values)`, reject empty or nil `names`, and reject more than 1024 pairs, each with `redis:issue?` and MUST NOT dial. The client MUST send `EX` or `EXAT`; it MUST NOT omit expiration and MUST NOT send NX, XX, PX, PXAT, or KEEPTTL. Integer reply `1` SHALL return no error. Integer reply `0` SHALL return `redis:issue?`. A `:` payload that is not a signed integer, or any integer other than `1` or `0`, SHALL return `redis:issue?`. AUTH-class prefixes still map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text unless they are unknown-command (fallback below). Clustered engines need all keys in one hash slot (hash tags); the client MUST NOT hash-tag, split, or retry cross-slot. Zero or negative TTL values SHALL be passed through, same as `Set`.

On first `MSetEX`/`MSetEXAt` per client, the session SHALL send native `MSETEX`. If the error text has prefix `ERR unknown command`, the session SHALL cache Lua for that client, then run the fallback via `Eval` for that call. A cached Lua miss MUST NOT send `MSETEX` again. A cached native hit that later sees `ERR unknown command` SHALL recache Lua and `Eval` that call. The cache SHALL live on the client under a dedicated mutex, not on a pooled connection and not on `idleConnsMu`. A new `New`/value is a new cache.

The fallback SHALL be one `Eval` of a Lua 5.1-safe script: `numkeys` equal to `len(names)`, values then the token (`EX` or `EXAT`) then the TTL decimal in ARGV, loop `for i = 1, #KEYS do redis.call('SET', KEYS[i], ARGV[i], token, ttl) end`. The script MUST NOT call `unpack`, `table.unpack`, or `table.maxn`. Values and the TTL token MUST NOT be placed in KEYS. Integer `1` from that EVAL SHALL return no error. Past `EXAT` MAY delete keys and still succeed with `1`; the client MUST NOT hide that `1`. The fallback MUST call `Eval` (EVALSHA then EVAL on NOSCRIPT); it MUST NOT add a second script-load path.

Native argv tests SHALL run against the in-process fake only. Those tests MUST NOT require a Valkey or Redis 8 process.

#### Scenario: Native MSETEX argv
- **WHEN** MSetEX is called with two names, two values, and a positive duration against a fake that implements MSETEX
- **THEN** the command sent is `MSETEX`, `2`, those names and values in order, `EX`, and those seconds as decimal digits
- **AND** integer `1` returns no error

#### Scenario: MSetEXAt argv
- **WHEN** MSetEXAt is called with one name, one value, and a Unix timestamp against a fake that implements MSETEX
- **THEN** the command sent is `MSETEX`, `1`, that name, that value, `EXAT`, and that timestamp as decimal digits

#### Scenario: Integer 0 is redis:issue?
- **WHEN** native MSETEX replies integer `0`
- **THEN** MSetEX returns `redis:issue?`
- **AND** the error is not treated as success

#### Scenario: Empty and mismatched input do not dial
- **WHEN** MSetEX is called with no names, or with a different number of values than names, or with 1025 pairs
- **THEN** each call returns `redis:issue?`
- **AND** no TCP connection is opened

#### Scenario: Unknown-command falls back to EVAL and caches
- **WHEN** MSetEX is called against a fake that replies `-ERR unknown command 'MSETEX'`
- **THEN** that call succeeds via EVAL of the fallback script with the names in KEYS
- **AND** a second MSetEX on the same client does not send `MSETEX`
- **AND** the EVAL argv has `numkeys` equal to the name count, then those names, then the values, then `EX`, then the TTL

#### Scenario: Lua loop is 5.1-safe and declares KEYS
- **WHEN** the fallback script is sent
- **THEN** the script body uses `for i = 1, #KEYS` and `redis.call('SET', KEYS[i], ARGV[i], token, ttl)`
- **AND** the body does not contain `unpack`, `table.unpack`, or `table.maxn`

### Requirement: Live Redis and Dragonfly prove Lua MSetEX TTL landed
Compiled tests SHALL run `MSetEX` against both Redis 7 and Dragonfly when `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY` are set. Those tests MUST skip under `-short` or when both addresses are unset. After `MSetEX`, `Get` SHALL return the written bytes and `Eval` of `TTL` on a declared KEYS key SHALL return a positive integer. `MSetEXAt` with a past timestamp SHALL then `Get` as a miss. CI MUST set both env vars to the existing Redis 7 `:6379` and Dragonfly `:6380` services. Pester on `/redis` and `/dragonfly` is not a substitute for this compiled live file.

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

#### Scenario: Live past EXAT is a miss
- **WHEN** either live address is set
- **AND** MSetEXAt is called with a Unix timestamp in the past
- **THEN** a later Get of that key is `redis:miss`

### Requirement: ExecPipeline sends N commands then reads N ordered slots
`ExecPipeline(commands [][][]byte)` SHALL encode each command as one RESP array of bulk strings on one borrowed connection, flush the socket once after those encodes, and read exactly N replies in order under one I/O deadline. The return SHALL be `([]PipelineSlot, error)` where `PipelineSlot` has `Values [][]byte` and `Err error`. When every reply was read, the slice length SHALL be N. Empty or nil `commands` SHALL return nil, nil and MUST NOT dial. A batch longer than 64 SHALL return an error whose `Error()` text is `redis:issue?` and MUST NOT send. The batch error SHALL be only I/O, protocol, cap, unreachable, or timeout. A `-` reply, `redis:miss`, or `redis:noauth` SHALL populate that slot’s `Err` via the same mapping as other commands and MUST NOT fail the batch error and MUST NOT stop remaining reads. Callers SHALL match `Error()` text and MUST NOT type-assert. The client MUST NOT export a `Pipeline` builder type. EVAL rows in the batch SHALL still list KEYS; scripts SHALL stay Lua 5.1-safe. `ExecPipeline` SHALL NOT hash or rewrite EVAL rows into EVALSHA; that remains `Eval`. `ExecPipeline` SHALL NOT replace `MSetEX`: a same-TTL group write stays native MSETEX or the Lua `Eval` fallback.

#### Scenario: N commands one flush ordered replies
- **WHEN** ExecPipeline is called with several small commands against a fake that counts client Writes
- **THEN** the client performs one flush (one coalesced Write)
- **AND** the returned slots are those replies in command order

#### Scenario: Empty batch does not dial
- **WHEN** ExecPipeline is called with no commands
- **THEN** the result is `nil, nil`
- **AND** no TCP connection is opened

#### Scenario: Over cap is redis:issue? without sending
- **WHEN** ExecPipeline is called with more than 64 commands
- **THEN** the batch error is `redis:issue?`
- **AND** no command is written to the socket

#### Scenario: Element -ERR still returns the other slots
- **WHEN** ExecPipeline is called with 10 commands
- **AND** the server replies `-ERR` on element 3 and well-formed values on the others
- **THEN** the batch error is nil
- **AND** slot 3’s `Err` is that Redis error text
- **AND** the other nine slots carry their replies
- **AND** a later command on that client reuses the connection
