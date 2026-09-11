## ADDED Requirements

### Requirement: MSetEX and MSetEXAt write many keys with one shared TTL
`MSetEX(names, values, seconds)` SHALL send native Redis `MSETEX` with decimal `numkeys` equal to `len(names)`, then each name/value pair in order, then `EX` and that duration as decimal seconds. `MSetEXAt(names, values, unixSeconds)` SHALL send the same argv with `EXAT` and that Unix timestamp. Both SHALL require `len(names) == len(values)`, reject empty or nil `names`, and reject more than 1024 pairs, each with `redis:issue?` and MUST NOT dial. The client MUST send `EX` or `EXAT`; it MUST NOT omit expiration and MUST NOT send NX, XX, PX, PXAT, or KEEPTTL. Integer reply `1` SHALL return no error. Integer reply `0` SHALL return `redis:issue?`. A `:` payload that is not a signed integer, or any integer other than `1` or `0`, SHALL return `redis:issue?`. AUTH-class prefixes still map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text unless they are unknown-command (fallback below). Clustered engines need all keys in one hash slot (hash tags); the client MUST NOT hash-tag, split, or retry cross-slot. Zero or negative TTL values SHALL be passed through, same as `Set`.

On first `MSetEX`/`MSetEXAt` per client, the session SHALL send native `MSETEX`. If the error text has prefix `ERR unknown command`, the session SHALL cache Lua for that client, then run the fallback `EVAL` for that call. A cached Lua miss MUST NOT send `MSETEX` again. A cached native hit that later sees `ERR unknown command` SHALL recache Lua and `EVAL` that call. The cache SHALL live on the client under the existing mutex, not on a pooled connection. A new `Init`/value is a new cache.

The fallback SHALL be one `Eval` of a Lua 5.1-safe script: `numkeys` equal to `len(names)`, values then the token (`EX` or `EXAT`) then the TTL decimal in ARGV, loop `for i = 1, #KEYS do redis.call('SET', KEYS[i], ARGV[i], token, ttl) end`. The script MUST NOT call `unpack`, `table.unpack`, or `table.maxn`. Values and the TTL token MUST NOT be placed in KEYS. Integer `1` from that EVAL SHALL return no error. Past `EXAT` MAY delete keys and still succeed with `1`; the client MUST NOT hide that `1`. EVALSHA and SCRIPT LOAD MUST NOT be added.

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

## MODIFIED Requirements

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `Init`, `Get`, `Set`, `Del`, `Incr`, `Eval`, and `MSetEX` against a compiled fake TCP Redis. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. Yaegi SHALL cover both MSetEX paths: a fake that implements MSETEX (native), and a fake that rejects MSETEX so the first call falls back to EVAL and a second call does not send `MSETEX`.

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

#### Scenario: Yaegi MSetEX native
- **WHEN** interpreted code Inits a client to a compiled fake that implements MSETEX
- **AND** it calls MSetEX with one name and value
- **THEN** MSetEX returns no error
- **AND** Get of that name returns the written bytes

#### Scenario: Yaegi MSetEX Lua fallback
- **WHEN** interpreted code Inits a client to a compiled fake that replies unknown-command to MSETEX
- **AND** it calls MSetEX twice
- **THEN** the first call returns no error
- **AND** the second call does not send `MSETEX`

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt against that backend, and set one response header per verb. After MSetEX it SHALL Eval `TTL` on that key listed in KEYS and set a header to the positive TTL decimal. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new). Compose MUST keep the existing `/redis` and `/dragonfly` probe routes; this change MUST NOT add a Valkey or Redis 8 service.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEX-TTL that show those commands succeeded
- **AND** the MSetEX-TTL header is a positive decimal
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`
