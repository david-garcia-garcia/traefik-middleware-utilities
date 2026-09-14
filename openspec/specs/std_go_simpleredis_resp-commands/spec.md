## Purpose

Defines the RESP commands a SimpleRedis client speaks after it holds a session: GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL, MSetEX, and MSetEXAt, plus the exported error strings callers match. Keys and values are opaque bytes. Interpreter tests prove New/Get/Set/Del/Incr/Eval/MSetEX under Yaegi without starting Traefik.

## Requirements

### Requirement: Get returns the stored value or a miss
`Get(name)` SHALL send Redis `GET` for that key. A bulk reply SHALL return those bytes. After a one-slot reply, a nil slot (RESP2 null bulk `$-1`) SHALL return an error whose `Error()` text is `redis:miss`. Empty bulk `$0` SHALL return a non-nil empty slice and MUST NOT return `redis:miss`. After a peer writes an extra well-formed reply that is already in the connection reader at the reply boundary, Get MUST return the value for its own key or an error and MUST NOT return another key’s bytes. That guarantee does not cover an unsolicited reply that arrives only into the kernel receive buffer while the socket is idle.

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

#### Scenario: Empty bulk is not a miss
- **WHEN** Get receives empty bulk `$0`
- **THEN** Get returns a non-nil empty slice
- **AND** the error is not `redis:miss`

#### Scenario: Get after stray extra is own-key or error
- **WHEN** `PoolSize` is `1` and `MaxRetries` is `-1`
- **AND** a peer answers `GET kN` with `vN` and, in the same write, appends one extra well-formed bulk every 5th command
- **AND** Get is called for `k0` through at least `k11`
- **THEN** each Get returns the bytes for that key or an error
- **AND** no Get returns another key’s bytes or the stray bulk as a success

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
Callers SHALL match errors by `errors.Is` against the exported sentinel values (`ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait`, `ErrUnsupportedReply`) or the predicates `IsMiss`, `IsUnreachable`, and `IsPoolWait`. The session SHALL still export these exact strings for display and legacy text matching: `redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`, `redis:unsupported-reply`. AUTH-class Redis error prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text. `ErrPoolWait` SHALL wrap `ErrUnreachable` so `errors.Is` on the unreachable sentinel matches pool wait, while `IsPoolWait` still distinguishes pool saturation. A wrapped sentinel SHALL still match `errors.Is` and the corresponding predicate; `err.Error() ==` the token MUST NOT be the only supported match. Handshake AUTH or SELECT failures that are those sentinels SHALL match the same way for compiled callers and for Yaegi-interpreted callers. The session MUST NOT return a package-local wrapper whose `Unwrap` compiled `errors.Is` cannot see under Yaegi.

#### Scenario: Rejected auth is redis:noauth
- **WHEN** Redis replies `-NOAUTH`
- **THEN** the command returns `redis:noauth`

#### Scenario: Wrapped miss still matches the miss sentinel
- **WHEN** a miss sentinel is wrapped with `%w`
- **THEN** `errors.Is` and `IsMiss` still match
- **AND** `err.Error()` is not equal to `redis:miss`

#### Scenario: Pool wait matches unreachable and is distinct
- **WHEN** the error is the pool-wait sentinel
- **THEN** `errors.Is` matches the unreachable sentinel
- **AND** `IsPoolWait` is true for pool wait and false for a plain unreachable sentinel

#### Scenario: Unsupported reply is redis:unsupported-reply
- **WHEN** the peer replies with a well-framed type this client does not decode
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the error is not `redis:issue?`

#### Scenario: Handshake AUTH close matches unreachable compiled and interpreted
- **WHEN** a compiled caller issues a command against a peer that accepts TCP, reads AUTH, and closes with no reply
- **THEN** `IsUnreachable` is true
- **WHEN** Yaegi-interpreted code issues that same command against the same kind of peer
- **THEN** `IsUnreachable` is true
- **AND** `Error()` is `redis:unreachable`

#### Scenario: Handshake AUTH WRONGPASS matches ErrNoAuth compiled and interpreted
- **WHEN** a compiled caller issues a command against a fake that replies `-WRONGPASS` to AUTH
- **THEN** `errors.Is` matches `ErrNoAuth`
- **WHEN** Yaegi-interpreted code issues that same command against the same kind of fake
- **THEN** `errors.Is` matches `ErrNoAuth`
- **AND** `Error()` is `redis:noauth`

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `New`, `Get`, `Set`, `Del`, `Incr`, `Eval`, and `MSetEX` against a compiled fake TCP Redis, including the NOSCRIPT fallback path. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. Yaegi SHALL cover both MSetEX paths: a fake that implements MSETEX (native), and a fake that rejects MSETEX so the first call falls back to EVAL and a second call does not send `MSETEX`. Interpreted `clientprobe` SHALL also observe `errors.Is` against an exported sentinel and one of `IsMiss`, `IsUnreachable`, or `IsPoolWait`. Those tests SHALL also prove interpreted matchers on errors the package returns: handshake AUTH peer-close (`IsUnreachable`) and handshake AUTH-class (`errors.Is` against `ErrNoAuth`). A `%w` wrap of a sentinel alone MUST NOT be the only interpreted matcher coverage.

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

#### Scenario: Yaegi matches an exported sentinel
- **WHEN** interpreted code calls `errors.Is` on an exported SimpleRedis sentinel and one predicate
- **THEN** both matches succeed

#### Scenario: Yaegi matches handshake AUTH unreachable
- **WHEN** interpreted code constructs a client with New with a password against a compiled fake that accepts TCP, reads AUTH, and closes with no reply
- **THEN** `IsUnreachable` is true
- **AND** a compiled control against the same kind of peer also matches `IsUnreachable`

#### Scenario: Yaegi matches handshake AUTH noauth
- **WHEN** interpreted code constructs a client with New with a password against a compiled fake that replies `-WRONGPASS` to AUTH
- **THEN** `errors.Is` matches `ErrNoAuth`
- **AND** a compiled control against the same kind of fake also matches `ErrNoAuth`

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
An RESP array (`*`) SHALL accept each element whose head is `$` (bulk, including null bulk as a nil slot), `:` (integer payload bytes), or `+` (status payload bytes). If an element head is `*` or `-`, the client SHALL return `redis:unsupported-reply`. Nested arrays are out of scope. MGET callers MUST still observe only bulk slots from Redis MGET. A `:` or `+` slot SHALL be an independent copy of those payload bytes so a later read on the same connection cannot overwrite it.

#### Scenario: Mixed array elements
- **WHEN** Redis replies with an array that contains a bulk, an integer, and a status
- **THEN** the result has three slots with those payloads
- **WHEN** an array element is a nested array
- **THEN** the client returns `redis:unsupported-reply`

#### Scenario: Integer and status slots survive a later read
- **WHEN** an array reply stores a `:` payload and a `+` payload
- **AND** a later command is read on the same connection
- **THEN** those stored slots still equal the original payloads

### Requirement: Probe Config passes password and database to New
The nested SimpleRedis Traefik plugin Config SHALL include `Password` and `Database` with empty default. Traefik `New` SHALL pass Host, Password, and Database to `simpleredis.New`. Success labels on `/redis` and `/dragonfly` MUST keep host-only settings so New receives an empty password and an empty database. `DropHost` remains for the lost-reply relay.

#### Scenario: Success routes stay host-only
- **WHEN** Traefik loads the plugin for `/redis` and `/dragonfly`
- **THEN** those routes call `simpleredis.New` with no password and an empty database
- **AND** a request on each route still succeeds

### Requirement: Traefik e2e proves handshake AUTH and SELECT failures
Compose SHALL add sibling Redis and Dragonfly services with requirepass. Compose MUST NOT put requirepass on the existing `redis` and `dragonfly` services that serve `/redis` and `/dragonfly`. Whoami routes SHALL call `simpleredis.New` with a wrong password against those siblings and with database `99` against the existing unpassworded `redis` and `dragonfly`. A request on a wrong-password route SHALL return HTTP 502 whose body is `redis:noauth`. A request on a database-99 route SHALL return HTTP 502 whose body contains `ERR DB index is out of range`. Those tests MUST NOT stop `whoami-a` or `whoami-b`. Eval on the success routes SHALL remain the existing Lua 5.1-safe script that lists its key in KEYS.

#### Scenario: Pester wrong password on Redis and Dragonfly
- **WHEN** a request is made on the wrong-password whoami route for Redis and for Dragonfly
- **THEN** each response is HTTP 502
- **AND** each body is `redis:noauth`
- **AND** neither test stops `whoami-a` or `whoami-b`

#### Scenario: Pester SELECT 99 on Redis and Dragonfly
- **WHEN** a request is made on the database-99 whoami route for Redis and for Dragonfly
- **THEN** each response is HTTP 502
- **AND** each body contains `ERR DB index is out of range`
- **AND** neither test stops `whoami-a` or `whoami-b`

### Requirement: ScriptSHA1Hex is Redis sha1hex
The package SHALL export `ScriptSHA1Hex(script string) string`. It SHALL return the SHA-1 of the script bytes as lowercase 40-character hex (Redis `sha1hex`). The client MUST NOT keep a digest table or mutex for scripts. Tests that import Yaegi v0.16.1 SHALL prove interpreted code can call `ScriptSHA1Hex` under GOPATH with stdlib symbols only and `useunsafe` false.

#### Scenario: ScriptSHA1Hex is 40-char lowercase hex
- **WHEN** `ScriptSHA1Hex` is called with a Lua body
- **THEN** the result is 40 lowercase hex characters
- **AND** that value equals SHA-1 of those script bytes

#### Scenario: Yaegi ScriptSHA1Hex
- **WHEN** interpreted code calls `ScriptSHA1Hex` with a script body
- **THEN** the result equals the compiled `ScriptSHA1Hex` of that body

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(ctx, script, digest, keys, args)` SHALL have the public signature `Eval(ctx context.Context, script string, digest string, keys []string, args []string) ([][]byte, error)`. Callers SHALL pass the script body and the SHA-1 hex from `ScriptSHA1Hex` (or an equivalent Redis `sha1hex`). `Eval` MUST NOT hash the script body. `Eval` MUST NOT check that `digest` equals `ScriptSHA1Hex(script)`. `Eval` SHALL send Redis `EVALSHA`, the caller `digest`, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args. That EVAL is the only place the script body is sent to Redis/Dragonfly so the engine stores it. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The client MUST NOT keep a digest table or mutex for scripts. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a top-level null bulk (`$-1`, including Lua `return false`) SHALL be one nil-slot element with a nil error and MUST NOT be `redis:miss`; Eval MUST NOT remap `redis:miss` as a special case of that verb; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). A Lua indexed table SHALL decode only as a flat array whose elements are bulk strings or integers (or status). Nested tables and `{ err = "..." }` inside an array SHALL return `redis:unsupported-reply`. Scripts that return several values MUST wrap each slot with Lua `tostring` (or return numbers, which become integers). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

#### Scenario: Later Eval sends EVALSHA not the body
- **WHEN** Eval is called twice with the same script, that script’s `ScriptSHA1Hex` digest, one key, and two args against a fake that already has that digest
- **THEN** the second command sent is `EVALSHA`, that digest, `1`, that key, then those args
- **AND** the second argv MUST NOT include the script body

#### Scenario: First EVALSHA miss falls back to EVAL
- **WHEN** the first `EVALSHA` for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends `EVAL`, that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: After EVAL the digest hits
- **WHEN** EVAL has loaded that digest on the fake
- **AND** Eval is called again with the same script and that digest
- **THEN** the command sent is `EVALSHA` with that digest
- **AND** Eval succeeds without a second EVAL

#### Scenario: Two scripts two digests
- **WHEN** Eval is called with script A and its digest then with a different script B and its digest
- **THEN** the `EVALSHA` argv digests differ

#### Scenario: Eval uses the caller digest
- **WHEN** Eval is called with a script and a digest equal to `ScriptSHA1Hex` of that script
- **THEN** the `EVALSHA` argv digest is that caller string
- **AND** Eval MUST NOT replace it with a newly hashed value

#### Scenario: Eval integer reply
- **WHEN** Redis replies to EVALSHA or EVAL with a `:` integer
- **THEN** Eval returns one `[][]byte` element whose bytes are that decimal payload

#### Scenario: Eval empty keys
- **WHEN** Eval is called with a script, its digest, no keys, and no args
- **THEN** the command sent includes `numkeys` `0`

#### Scenario: Eval three bulk strings
- **WHEN** Redis replies to Eval with a three-element array of bulk strings
- **THEN** Eval returns those three payloads in order

#### Scenario: Eval nested array is unsupported
- **WHEN** Redis replies to Eval with a nested array
- **THEN** Eval returns `redis:unsupported-reply`
- **AND** the idle pool is empty
- **AND** the next command on that client dials a new socket

#### Scenario: Eval null bulk is not a miss
- **WHEN** Eval receives a top-level RESP2 null bulk `$-1` (Lua `return false`)
- **THEN** Eval returns one nil slot
- **AND** the error is not `redis:miss`

### Requirement: Eval and MSetEX fallback hops each have a full command budget
`Eval` SHALL send EVALSHA then, on a `NOSCRIPT` prefix, EVAL as two separate command executions. Each hop SHALL compute its own overall deadline at that hop’s entry equal to `now + CommandTimeout` unless the caller’s context deadline is sooner. The EVAL hop MUST NOT inherit remaining time from the EVALSHA hop. Sharing remaining time across those hops can starve EVAL after a slow EVALSHA.

`MSetEX` and `MSetEXAt` SHALL send native MSETEX as its own command execution. On `ERR unknown command`, the Lua fallback via `Eval` SHALL start with a full command budget (one or two further hops). Comments on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex` SHALL record that each hop binds its own overall deadline on purpose.

Compiled tests SHALL prove Eval NOSCRIPT fallback still succeeds and that MSetEX unknown-command fallback still succeeds. Those tests MUST NOT fail because elapsed time exceeds one public-command budget. This change MUST NOT bind one stacked deadline across hops and MUST NOT change `exec` deadline binding.

#### Scenario: Eval NOSCRIPT fallback still succeeds
- **WHEN** the first EVALSHA for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends EVAL with that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: MSetEX unknown-command fallback still succeeds
- **WHEN** the first native MSETEX receives `-ERR unknown command`
- **THEN** MSetEX runs the Lua fallback via Eval
- **AND** MSetEX returns no error
- **AND** the caller error is not unknown-command

### Requirement: Encoded RESP bytes match dest framing on both backends
The client SHALL encode each command as the same RESP array of bulk strings dest currently produces (`*` count, per-arg `$` length, payload, CRLF), including native `MSETEX` argv from `MSetEX` / `MSetEXAt`. Tests MUST run against both Redis and Dragonfly (both supported backends). After the encoder change, compose plus Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` SHALL prove every existing verb still works live on both engines: Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt. Eval SHALL remain a Lua 5.1-safe script that lists its key in KEYS. The probe Eval body and the Dragonfly compose pin MUST NOT be rewritten unless the encoder change forces it.

#### Scenario: Encoded GET matches dest framing
- **WHEN** GET is encoded for key `session:9f2c1ab4-user-token`
- **THEN** the wire bytes equal dest framing `*2\r\n$3\r\nGET\r\n$27\r\nsession:9f2c1ab4-user-token\r\n`

#### Scenario: Encoded MSETEX matches dest framing
- **WHEN** native MSETEX is encoded for two pairs `a`/`1` and `b`/`2` with EX 60
- **THEN** the wire bytes equal dest framing `*8\r\n$6\r\nMSETEX\r\n$1\r\n2\r\n$1\r\na\r\n$1\r\n1\r\n$1\r\nb\r\n$1\r\n2\r\n$2\r\nEX\r\n$2\r\n60\r\n`

#### Scenario: Every verb still works live on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis `redis:7-alpine` at `redis:6379`) and on `/dragonfly` (Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`) after the encoder change
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEX-TTL that show those commands succeeded
- **AND** Eval used a Lua 5.1-safe script that lists `KEYS[1]`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Newline value still round-trips
- **WHEN** Set stores a value that contains newline bytes
- **AND** Get is called for that key
- **THEN** Get returns those exact bytes

### Requirement: Encode benches guard compiled allocations and Yaegi strategies
Compiled tests SHALL include `BenchmarkEncodeGet`, `BenchmarkEncodeEval`, and `BenchmarkEncodeMSetEX` that encode on the production encoder. Interpreter tests SHALL include `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` as strategy probes. Production encode SHALL match the single-write strategy those Yaegi benches measure.

#### Scenario: Compiled encode benches exist
- **WHEN** package `simpleredis` encode benches run
- **THEN** `BenchmarkEncodeGet`, `BenchmarkEncodeEval`, and `BenchmarkEncodeMSetEX` execute against the production encoder

#### Scenario: Yaegi encode strategy benches exist
- **WHEN** Yaegi encode benches run
- **THEN** `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` execute as interpreted strategy probes
- **AND** production encode matches the single-write strategy

### Requirement: Interpreter tests assert the unsafe conversion matrix
Tests that import Yaegi SHALL assert which `string`/`[]byte` conversions the interpreter accepts under stdlib-only symbols, stdlib plus unsafe symbols, and unrestricted. Those tests MAY register Yaegi unsafe symbols and MAY import `unsafe` in `_test.go` files. Existing Init/Get/Set/Del/Incr/Eval/MSetEX interpreter tests MUST still use GOPATH with stdlib symbols only and `useunsafe` false. Named copy-versus-unsafe benches SHALL exist so a human can reproduce the measured ns/op; they MUST NOT fail `go test` without `-bench`. Those tests MUST NOT start Traefik.

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
Compiled tests SHALL fail when a non-test file in the SimpleRedis session folder imports `unsafe` or `"C"`, or a non-stdlib dotted path. Compiled tests SHALL fail when the SimpleRedis probe plugin manifest or compose `simpleredisprobe` `useUnsafe` is true. Absent or false on the manifest SHALL pass. Those tests MUST NOT require an explicit `useUnsafe: false` on the manifest. Those tests MUST NOT scan the reclaim probe. Redis and Dragonfly Pester proofs of existing verbs MUST stay. Eval scripts that touch keys MUST list those keys (Dragonfly). Lua MUST stay 5.1-safe.

#### Scenario: Session source import scan
- **WHEN** compiled tests list imports of non-test files in the SimpleRedis session folder
- **THEN** the test fails if any import path is `unsafe` or `"C"` or contains a dot
- **AND** `_test.go` files MAY import `unsafe`

#### Scenario: Probe useUnsafe scan
- **WHEN** compiled tests read the SimpleRedis probe Traefik manifest and the compose `simpleredisprobe` `useunsafe` setting
- **THEN** the test passes if the manifest field is absent or false and compose is false
- **AND** the test fails if either is true
- **AND** the reclaim probe is not scanned

### Requirement: Malformed RESP is a protocol issue and is not pooled
A line that does not end in CR before LF, an empty line, an unparseable `*` count, an `*` count less than 0, a truncated array element or bulk, or a complete bulk payload whose two trailer bytes are not CR then LF SHALL return an error whose `Error()` text is `redis:issue?`, or an I/O error (`redis:unreachable` on EOF, `redis:timeout` on deadline). That connection MUST NOT re-enter the idle pool. A wrong bulk trailer MUST return `redis:issue?` and MUST NOT return `redis:unreachable`. Unknown type bytes, nested arrays, and array elements whose type is not `$`, `:`, or `+` are `redis:unsupported-reply` (requirement Unsupported RESP replies are distinguishable and are not pooled), not `redis:issue?`.

#### Scenario: Missing CR before LF
- **WHEN** a reply line ends in LF without a preceding CR
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Empty line
- **WHEN** the peer replies with a CRLF-only line
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Unparseable array count
- **WHEN** the peer replies with `*` followed by a payload that is not a signed integer
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Null array is not a miss
- **WHEN** the peer replies with RESP2 null array `*-1`
- **THEN** the command returns `redis:issue?`
- **AND** the error is not `redis:miss`
- **AND** the idle pool is empty

#### Scenario: Truncated array or bulk is I/O
- **WHEN** the peer writes a partial array or bulk and closes the socket
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Truncated bulk after a complete ReadSlice head returns own-value on the next Get
- **WHEN** `MaxRetries` is `-1`
- **AND** a Get receives `$100\r\n`, then 40 bytes, then a close
- **THEN** that Get returns `redis:unreachable`
- **AND** the idle pool is empty
- **WHEN** a later Get receives a complete bulk of known bytes
- **THEN** that Get returns those bytes

#### Scenario: Retry after a dirty reused connection cannot dial
- **WHEN** a reused idle connection is dirtied by a truncated reply
- **AND** the retry dial fails
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Wrong bulk trailer is issue and not pooled
- **WHEN** a Get receives a complete `$` payload whose two trailer bytes are not CR then LF
- **THEN** the command returns `redis:issue?`
- **AND** the error is not `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Second Get after a wrong bulk trailer returns its own value
- **WHEN** `MaxRetries` is `-1`
- **AND** `PoolSize` is `1`
- **AND** a Get receives a complete bulk whose trailer is not CRLF and leftover bytes remain on that connection
- **THEN** that Get returns `redis:issue?`
- **AND** the idle pool is empty
- **WHEN** a later Get receives a complete bulk of known bytes
- **THEN** that Get returns those bytes
- **AND** those bytes are not remnants of the first payload

### Requirement: Over-cap bulk or array header is redis:issue?
A `$` bulk length greater than `64 << 20` or a `*` array count greater than `1 << 20` SHALL return an error whose `Error()` text is `redis:issue?`. That connection MUST NOT re-enter the idle pool. The command MUST NOT retry that error. A truncated bulk or array whose announced size is at or under those ceilings SHALL still be I/O (`redis:unreachable` on EOF). An over-cap header with no payload MUST NOT take that truncated I/O path.

#### Scenario: Over-cap bulk Get is redis:issue?
- **WHEN** Get receives a `$` header whose length is greater than `64 << 20` and no payload
- **THEN** Get returns `redis:issue?`
- **AND** the idle pool is empty
- **AND** the error is not `redis:unreachable`

#### Scenario: Over-cap array is redis:issue?
- **WHEN** the peer replies with a `*` header whose count is greater than `1 << 20`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: MGET-shaped over-cap bulk element is redis:issue?
- **WHEN** MGet receives an array whose `$` element length is greater than `64 << 20`
- **THEN** MGet returns `redis:issue?`
- **AND** the idle pool is empty

### Requirement: Unsupported RESP replies are distinguishable and are not pooled
A well-framed reply whose type byte is not `+`, `-`, `:`, `$`, or `*` (including an HTTP-shaped first line and RESP3 type bytes `_`, `#`, `,`, `(`, `%`, `~`, `=`, `>`), or an array element whose type is not `$`, `:`, or `+` (including a nested array or a `-` error inside an array), SHALL return an error whose `Error()` text is `redis:unsupported-reply`. That error MUST NOT be `redis:issue?` and MUST NOT be `redis:unreachable` solely because the type was unsupported. That connection MUST NOT re-enter the idle pool. A following command on the same client SHALL dial a new socket.

#### Scenario: Unknown type including HTTP-shaped
- **WHEN** the peer replies with a line whose first byte is not `+`, `-`, `:`, `$`, or `*`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: RESP3 type byte
- **WHEN** the peer replies with a RESP3 type byte `_`, `#`, `,`, `(`, `%`, `~`, `=`, or `>`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Bad array element type
- **WHEN** an array element’s type byte is not `$`, `:`, or `+`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Nested array is not pooled
- **WHEN** an array element is a nested array
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Error inside an array
- **WHEN** an array element is a Redis error line
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

### Requirement: Get and integer verbs reject wrong reply arity
`Get` SHALL return `redis:issue?` when a well-formed reply has a value count other than 1. `Incr` and `IncrBy` SHALL return `redis:issue?` when a well-formed reply has a value count other than 1. Those commands MUST NOT destroy the connection solely because the count mismatched; a successful decode MAY re-enter the idle pool.

#### Scenario: Get empty array
- **WHEN** Get receives a well-formed empty array `*0`
- **THEN** Get returns `redis:issue?`
- **AND** the connection remains in the idle pool

#### Scenario: Get extra elements
- **WHEN** Get receives a well-formed array of two bulks
- **THEN** Get returns `redis:issue?`
- **AND** the connection remains in the idle pool

#### Scenario: Incr empty array
- **WHEN** Incr receives a well-formed empty array `*0`
- **THEN** Incr returns `redis:issue?`
- **AND** the connection remains in the idle pool

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

### Requirement: Public verbs take a context
Each public command (`Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`) SHALL take `context.Context` as its first argument. There SHALL NOT be a matching `*Context` twin or an unadorned method that wraps `context.Background()`. A caller with no deadline SHALL pass `context.Background()` at the call site. Wire behavior SHALL include the session overall deadline and zero-Config defaults specified on `std_go_simpleredis_tcp-session`.

#### Scenario: Get sends GET
- **WHEN** Get is called with a context and a key
- **THEN** the session sends Redis GET for that key
- **AND** a bulk reply returns those bytes

#### Scenario: Already-cancelled Get does not send
- **WHEN** Get is called with a context that is already cancelled
- **THEN** the call returns that context's `Err()`
- **AND** the session MUST NOT send GET

### Requirement: SimpleRedis test package compiles
`go test ./simpleredis/` SHALL compile as one binary. Tests that write a temp GOPATH for Yaegi, including interpreted-cost measurements, MUST use the same-package helper the interpreter tests already define. The package MUST NOT fail to compile because that helper is missing. CI `go test ./...` MUST keep compiling this package.

#### Scenario: Test binary compiles
- **WHEN** `go test -c ./simpleredis/` runs
- **THEN** the compile succeeds

### Requirement: Keys and values with CRLF are data, not commands
The encoder SHALL write every argument as a length-prefixed RESP bulk string. A key and a value that each contain CRLF and an inline `PING` command payload MUST round-trip as stored data. A peer that parses strict RESP MUST treat those bytes as one argument and MUST NOT execute an injected command.

#### Scenario: CRLF and inline PING round-trip as data
- **WHEN** Set stores a key and a value that each contain CRLF and an inline PING payload
- **AND** Get reads that key from a strict-RESP fake
- **THEN** Get returns the same value bytes
- **AND** the fake did not treat the payload as a second command

### Requirement: Concurrent MSetEX unknown-command fallback stays balanced
Compiled unit tests SHALL run many overlapping MSetEX calls against a fake that rejects MSETEX. Those calls MUST succeed. After they finish, the in-use-turn channel SHALL be full and extra turn returns SHALL be zero.

#### Scenario: Sixteen overlapping MSetEX fallbacks
- **WHEN** many goroutines each call MSetEX repeatedly against a reject-MSETEX fake
- **THEN** every call returns no error
- **AND** the in-use-turn channel is full
- **AND** extra turn returns are zero

### Requirement: Interpreter tests observe error paths
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can run the dial-failure retry loop (including backoff), map an I/O timeout against a stalling peer, cancel a command in flight, wait on a full pool, and handle a truncated bulk, using GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. They MUST NOT panic in the interpreter. They MUST NOT assert `IsUnreachable` or `errors.Is` on AUTH or SELECT handshake failures. They MUST live in a test file that is not the existing happy-path Yaegi file.

#### Scenario: Interpreted dial failure retries without panic
- **WHEN** interpreted code Gets against a host that refuses the first dials
- **THEN** the interpreter does not panic
- **AND** the call returns an error the compiled tests already classify for that path

#### Scenario: Interpreted stall maps to timeout
- **WHEN** interpreted code Gets against a peer that accepts TCP and never replies
- **THEN** the interpreter does not panic
- **AND** the error text is `redis:timeout`

#### Scenario: Interpreted cancel, pool wait, and truncated bulk
- **WHEN** interpreted code cancels a command, waits on a full pool, or reads a truncated bulk
- **THEN** the interpreter does not panic
- **AND** each path returns the same classification the compiled suite already asserts
