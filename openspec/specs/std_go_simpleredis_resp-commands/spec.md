## Purpose

Defines the RESP commands a SimpleRedis client speaks after it holds a session: GET, MGET, SET with EX, and DEL, plus the exported error strings callers match. Keys and values are opaque bytes. Interpreter tests prove Init/Get/Set/Del under Yaegi without starting Traefik.

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
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `Init`, `Get`, `Set`, and `Del` against a compiled fake TCP Redis. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi Init Get Set Del
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it Sets a key and Gets that key
- **AND** it Dels that key
- **THEN** Get returns the bytes that were Set
- **AND** a later Get of that key is a miss or the Del returned no error

### Requirement: Traefik request SET plus GET sets a response header
A request through the nested SimpleRedis Traefik plugin SHALL SET a key and GET it back, then set a response header from that GET so Pester can assert the round-trip. Compose Redis SHALL be `redis:7-alpine` at `redis:6379` with no password and an empty database.

#### Scenario: Pester asserts the GET header
- **WHEN** a request is made on the plugin’s whoami route `/redis`
- **THEN** the response includes a header whose value is the bytes GET returned after SET
- **AND** the Redis Pester Describe does not stop `whoami-a` or `whoami-b`
