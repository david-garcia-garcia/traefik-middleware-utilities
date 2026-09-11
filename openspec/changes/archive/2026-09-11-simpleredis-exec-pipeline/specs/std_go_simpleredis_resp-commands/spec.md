## ADDED Requirements

### Requirement: ExecPipeline sends N commands then reads N ordered slots
`ExecPipeline(commands [][][]byte)` SHALL encode each command as one RESP array of bulk strings on one borrowed connection, flush the socket once after those encodes, and read exactly N replies in order under one I/O deadline. The return SHALL be `([]PipelineSlot, error)` where `PipelineSlot` has `Values [][]byte` and `Err error`. When every reply was read, the slice length SHALL be N. Empty or nil `commands` SHALL return nil, nil and MUST NOT dial. A batch longer than 64 SHALL return an error whose `Error()` text is `redis:issue?` and MUST NOT send. The batch error SHALL be only I/O, protocol, cap, unreachable, or timeout. A `-` reply, `redis:miss`, or `redis:noauth` SHALL populate that slot’s `Err` via the same mapping as other commands and MUST NOT fail the batch error and MUST NOT stop remaining reads. Callers SHALL match `Error()` text and MUST NOT type-assert. The client MUST NOT export a `Pipeline` builder type. EVAL rows in the batch SHALL still list KEYS; scripts SHALL stay Lua 5.1-safe.

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

## MODIFIED Requirements

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `Init`, `Get`, `Set`, `Del`, `Incr`, `Eval`, and `ExecPipeline` against a compiled fake TCP Redis. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik.

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

#### Scenario: Yaegi ExecPipeline
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it calls ExecPipeline with INCR then GET of that key
- **THEN** the batch error is nil
- **AND** the INCR slot value is `1`
- **AND** the GET slot carries those bytes

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. After those verbs, the same request SHALL call ExecPipeline with INCR, EXPIRE, GET of that incr key, and EVAL of a Lua 5.1-safe Kong INCRBY+EXPIREAT snippet that lists `KEYS[1]` on a distinct per-request eval key, and SHALL set `X-SimpleRedis-Pipeline` to `1:ok:1:3`. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Compose MUST NOT add a new route, engine, or `--pipeline_queue_limit` flag. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new). Live mixed-verb pipeline proof on Redis and Dragonfly is REQUIRED; a flush-counting fake MUST NOT substitute for it.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts the pipeline header on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes `X-SimpleRedis-Pipeline` with value `1:ok:1:3`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`
