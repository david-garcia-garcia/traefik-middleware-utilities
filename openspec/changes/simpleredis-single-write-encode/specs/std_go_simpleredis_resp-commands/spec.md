## ADDED Requirements

### Requirement: Encoded RESP bytes match dest framing on both backends
The client SHALL encode each command as the same RESP array of bulk strings dest currently produces (`*` count, per-arg `$` length, payload, CRLF). Tests MUST run against both Redis and Dragonfly (both supported backends). After the encoder change, compose plus Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` SHALL prove every existing verb still works live on both engines: Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval. Eval SHALL remain a Lua 5.1-safe script that lists its key in KEYS. The probe Eval body and the Dragonfly compose pin MUST NOT be rewritten unless the encoder change forces it.

#### Scenario: Encoded GET matches dest framing
- **WHEN** GET is encoded for key `session:9f2c1ab4-user-token`
- **THEN** the wire bytes equal dest framing `*2\r\n$3\r\nGET\r\n$28\r\nsession:9f2c1ab4-user-token\r\n`

#### Scenario: Every verb still works live on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis `redis:7-alpine` at `redis:6379`) and on `/dragonfly` (Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`) after the encoder change
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval that show those commands succeeded
- **AND** Eval used a Lua 5.1-safe script that lists `KEYS[1]`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Newline value still round-trips
- **WHEN** Set stores a value that contains newline bytes
- **AND** Get is called for that key
- **THEN** Get returns those exact bytes

### Requirement: Encode benches guard compiled allocations and Yaegi strategies
Compiled tests SHALL include `BenchmarkEncodeGet` and `BenchmarkEncodeEval` that encode on the production encoder. Interpreter tests SHALL include `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` as strategy probes. Production encode SHALL match the single-write strategy those Yaegi benches measure.

#### Scenario: Compiled encode benches exist
- **WHEN** package `simpleredis` encode benches run
- **THEN** `BenchmarkEncodeGet` and `BenchmarkEncodeEval` execute against the production encoder

#### Scenario: Yaegi encode strategy benches exist
- **WHEN** Yaegi encode benches run
- **THEN** `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` execute as interpreted strategy probes
- **AND** production encode matches the single-write strategy
