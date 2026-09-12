## ADDED Requirements

### Requirement: Malformed RESP is a protocol issue and is not pooled
A reply whose type byte is not `+`, `-`, `:`, `$`, or `*` (including an HTTP-shaped first line), a line that does not end in CR before LF, an empty line, an unparseable `*` count, an `*` count less than 0, a truncated array element or bulk, or an array element whose type is not `$`, `:`, or `+` SHALL return an error whose `Error()` text is `redis:issue?`, or an I/O error (`redis:unreachable` on EOF, `redis:timeout` on deadline). That connection MUST NOT re-enter the idle pool.

#### Scenario: Unknown type including HTTP-shaped
- **WHEN** the peer replies with a line whose first byte is not `+`, `-`, `:`, `$`, or `*`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

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

#### Scenario: Bad array element type
- **WHEN** an array element’s type byte is not `$`, `:`, or `+`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Truncated array or bulk is I/O
- **WHEN** the peer writes a partial array or bulk and closes the socket
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Nested array is not pooled
- **WHEN** an array element is a nested array
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Retry after a dirty reused connection cannot dial
- **WHEN** a reused idle connection is dirtied by a malformed reply
- **AND** the retry dial fails
- **THEN** the command returns `redis:unreachable`
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

## MODIFIED Requirements

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. The same request SHALL also call MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval against that backend, and set one response header per verb. The same request SHALL Get a missing key and set a response header whose value is `redis:miss`. Compose SHALL include `redis:7-alpine` at `redis:6379` with no password and an empty database, and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379` with no password and an empty database. Eval SHALL send a Lua 5.1-safe script that lists its key in KEYS (INCRBY plus EXPIREAT when the key is new).

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts every verb on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Pester asserts Get-miss on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis) and on `/dragonfly` (Dragonfly)
- **THEN** each response includes a header whose value is `redis:miss`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`
