## ADDED Requirements

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(script, keys, args)` SHALL keep the public signature `Eval(script string, keys []string, args []string) ([][]byte, error)`. Callers pass the script body; they MUST NOT pass a digest. The client SHALL cache the SHA-1 of each distinct script as lowercase hex. `Eval` SHALL send Redis `EVALSHA`, that digest, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args, then keep using the digest. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

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

## MODIFIED Requirements

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `Init`, `Get`, `Set`, `Del`, `Incr`, and `Eval` against a compiled fake TCP Redis, including the NOSCRIPT fallback path. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik.

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

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. Eval SHALL run twice on that request: `X-SimpleRedis-Eval` from the first result, `X-SimpleRedis-EvalAgain` from the second. The probe SHALL set `X-SimpleRedis-EvalDigest` to the SHA-1 hex of the Kong KEYS snippet const. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new) and MUST NOT use `table.maxn`. Pester SHALL prove EVALSHA + NOSCRIPT fallback live on both engines: `SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest is `0`, GET succeeds (`Eval` `3` and `EvalAgain` `3`), `EXISTS` is `1`, GET again succeeds. Same sequence against Dragonfly via `redis-cli -h dragonfly`. Existing verb headers stay. Reclaim `/a` `/b` stay up.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
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

## REMOVED Requirements

### Requirement: Eval sends EVAL with numkeys equal to the key count
**Reason**: Eval now sends EVALSHA of a client SHA-1 digest and falls back once to EVAL on NOSCRIPT. The public signature is unchanged.
**Migration**: Keep calling `Eval(script, keys, args)`. Do not call EVALSHA from tokenbucket or windowcounter. Do not rewrite `std_go_tokenbucket_lua-eval`.
