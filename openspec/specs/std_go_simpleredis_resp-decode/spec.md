## Purpose

Defines how SimpleRedis decodes RESP lines: `ReadSlice` buffer views, copy-on-escape for status and integer payloads, length parse from bytes, compiled proof for aliasing and long lines, and live Get/MGet/Incr/Eval remaining correct on Redis and Dragonfly.

## Requirements

### Requirement: RESP lines use ReadSlice and recover from a full buffer
The session SHALL read each RESP line with `bufio.Reader.ReadSlice('\n')`. When that call returns `bufio.ErrBufferFull`, the session SHALL return `redis:issue?` and MUST NOT read the remainder with `ReadBytes`. Any other `ReadSlice` error SHALL be returned to the caller. After a complete line, the session SHALL strip CRLF; a line shorter than two bytes or missing `\r` before `\n` SHALL return `redis:issue?`. The session MUST NOT loop `ReadSlice` until success. The session MUST NOT enlarge the reader past `bufio.NewReader` default size 4096. The session MUST NOT import `go-redis`.

#### Scenario: Short line
- **WHEN** a RESP line is shorter than 4096 bytes
- **THEN** the decoder returns that line without the CRLF

#### Scenario: Line longer than 4096 bytes
- **WHEN** a status or error line is longer than 4096 bytes
- **THEN** the command returns `redis:issue?`
- **AND** the caller does not receive `bufio.ErrBufferFull`
- **AND** the socket is not returned to the idle pool

#### Scenario: Unterminated stream fills the buffer
- **WHEN** the peer writes more than 4096 bytes with no newline
- **THEN** the command returns `redis:issue?`
- **AND** the decoder does not consume more than 4096 bytes from the peer

### Requirement: Escaping plus and integer payloads are copies
When a `+` or `:` payload is returned to the caller or stored in an array slot, the session SHALL copy those bytes before the next read on that connection and before the connection is released to the idle pool. Header-only uses (type byte, length parse) MUST NOT keep the `ReadSlice` view across a later read. A `-` error payload MAY rely on converting the message to `string` (that already copies). Bulk `$` payloads stay owned by the decoder's allocated buffer. After a complete bulk of announced length plus trailer is read, the last two bytes SHALL be CR then LF; otherwise the session SHALL return `redis:issue?`. Empty bulk `$0` whose trailer is CRLF SHALL succeed. A bulk length above the package bulk ceiling MUST NOT allocate a payload buffer.

#### Scenario: Held payload survives a later command
- **WHEN** a command returns a `+` or `:` payload
- **AND** the caller holds that slice
- **AND** a second command on the same connection returns a distinct `+` or `:` payload
- **THEN** the first slice still equals the first payload

#### Scenario: Wrong bulk trailer is issue
- **WHEN** a complete bulk payload of announced length is read
- **AND** the two bytes after that payload are not CR then LF
- **THEN** the session returns `redis:issue?`

#### Scenario: Empty bulk with CRLF trailer succeeds
- **WHEN** the peer replies `$0\r\n\r\n`
- **THEN** the session returns empty bytes
- **AND** the error is not `redis:issue?`

### Requirement: Lengths parse from header bytes
Bulk and array header lengths SHALL be parsed from the bytes after the type byte. An optional leading minus SHALL be accepted so `$-1` and `*-1` parse as negative lengths. A top-level `$` whose length is less than 0 SHALL return one nil slot and a nil error; that reply MUST NOT be `redis:miss`. Empty remainder or non-digit bytes SHALL return `redis:issue?`. The parse MUST NOT use `unsafe`. The parse MUST NOT import `go-redis`.

#### Scenario: Top-level null bulk is a nil slot
- **WHEN** the peer replies `$-1`
- **THEN** the decoder returns one nil slot
- **AND** the error is not `redis:miss`

#### Scenario: Garbage length is issue
- **WHEN** the array or bulk header length is empty or not digits
- **THEN** the command returns `redis:issue?`

### Requirement: Bulk and array headers above package ceilings do not allocate
The decoder SHALL reject a bulk length greater than `64 << 20` bytes and an array element count greater than `1 << 20` after a successful length parse and after the bulk-miss check. Those ceilings MUST be package constants, not session `Config` fields. An over-ceiling bulk or array header SHALL return `redis:issue?` and MUST NOT allocate a payload buffer or an array of that announced size. Array `$` elements SHALL use the same bulk ceiling as a top-level bulk. Lengths at or under the ceilings MUST NOT be treated as an allocation size before payload or elements arrive. `parseLen` overflow (a digit string that does not fit in `int`) SHALL remain `redis:issue?` without a wrap.

#### Scenario: Bulk just over the ceiling is issue
- **WHEN** the peer replies with a `$` header whose length is one more than `64 << 20`
- **AND** no payload bytes follow
- **THEN** the decoder returns `redis:issue?`
- **AND** the stream is not reusable

#### Scenario: Array just over the ceiling is issue
- **WHEN** the peer replies with a `*` header whose count is one more than `1 << 20`
- **THEN** the decoder returns `redis:issue?`
- **AND** the stream is not reusable

#### Scenario: MaxInt64 bulk header is issue
- **WHEN** the peer replies with a `$` header whose digits are the decimal of MaxInt64
- **THEN** the decoder returns `redis:issue?`
- **AND** the stream is not reusable

#### Scenario: 256 MiB bulk header does not allocate that payload
- **WHEN** the peer replies with `$268435456` and no payload
- **THEN** the decoder returns `redis:issue?`
- **AND** the decoder does not allocate a 268435456-byte payload buffer

#### Scenario: Array bulk element inherits the bulk ceiling
- **WHEN** the peer replies with a one-element array whose `$` element length is greater than `64 << 20`
- **THEN** the decoder returns `redis:issue?`
- **AND** the stream is not reusable

#### Scenario: Overflow digit string remains issue
- **WHEN** a bulk or array length is a digit string longer than `int` can hold
- **THEN** the decoder returns `redis:issue?`

### Requirement: In-cap bulk and array headers allocate from arrived bytes
For a bulk length at or under the package bulk ceiling, the decoder MUST NOT allocate a payload buffer of the announced size before payload bytes arrive. Memory for that payload SHALL stay proportional to bytes that actually arrived, and MUST NOT exceed the bulk ceiling. For an array count at or under the package array ceiling, the decoder MUST NOT allocate a slot slice of the announced count before elements arrive; the slot slice SHALL grow as elements decode. A `$` header equal to the bulk ceiling with no following payload, and a `*` header equal to the array ceiling with no following element line, MUST NOT allocate a buffer or slot slice of that announced size. A short read of an incomplete payload SHALL remain `redis:unreachable`. Over-ceiling headers SHALL remain `redis:issue?`.

#### Scenario: In-cap bulk header with no payload does not allocate that payload
- **WHEN** the peer replies with a `$` header whose length is `64 << 20`
- **AND** no payload bytes follow
- **THEN** the decoder does not allocate a payload buffer of that announced size
- **AND** the decoder returns `redis:unreachable`

#### Scenario: In-cap array header with no elements does not allocate that slot slice
- **WHEN** the peer replies with a `*` header whose count is `1 << 20`
- **AND** no element lines follow
- **THEN** the decoder does not allocate a slot slice of that announced count
- **AND** the decoder returns `redis:unreachable`

#### Scenario: Complete small bulk still succeeds
- **WHEN** the peer replies `$17` plus seventeen payload bytes and a CRLF trailer
- **THEN** the decoder returns those seventeen bytes
- **AND** the error is not `redis:issue?`

### Requirement: Copy-on-escape and ErrBufferFull have compiled unit tests
Compiled tests in `simpleredis/` SHALL prove a held `+` or `:` slice stays correct after a later read on the same connection, using distinct payloads. Those tests MUST NOT use identical EVAL `:0` replies that hide aliasing. Compiled tests SHALL also prove a status or error line longer than 4096 bytes returns `redis:issue?` and is not pooled, and that an unterminated stream that fills the 4096-byte buffer returns `redis:issue?` without consuming more than 4096 bytes from the peer. Those tests MUST NOT require live Redis or Dragonfly. Those tests MUST NOT use `runtime.ReadMemStats`.

#### Scenario: Unit tests cover aliasing and long lines
- **WHEN** `go test ./simpleredis/...` runs
- **THEN** a test fails if a held `+` or `:` slice is overwritten by the next read
- **AND** a test fails if a line longer than 4096 bytes is decoded
- **AND** a test fails if an unterminated stream that fills the buffer is decoded or if more than 4096 bytes were consumed from the peer

### Requirement: Decode allocation benches exist
`simpleredis/bench_test.go` SHALL include `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, and `BenchmarkDecodeInteger` against compiled fake RESP. Those benches MUST NOT dial live Redis or Dragonfly.

#### Scenario: Named decode benches compile
- **WHEN** `go test ./simpleredis/ -bench=BenchmarkDecode -benchmem` runs
- **THEN** those three benches run

### Requirement: Live Get MGet Incr Eval stay correct on Redis and Dragonfly
After this decode change, a request through `e2e/simpleredisprobe` SHALL still succeed for Get, MGet, Incr, and Eval on both compose backends. Tests MUST run against Redis (`/redis`, `redis:7-alpine` at `redis:6379`) and against Dragonfly (`/dragonfly`, `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`) via Pester. Eval scripts SHALL stay Lua 5.1-safe and list touched keys in KEYS (Dragonfly rejects undeclared keys). This change MUST NOT edit probe, tokenbucket, or windowcounter scripts. This change MUST NOT add compose services.

#### Scenario: Pester Get MGet Incr Eval on both engines
- **WHEN** compose is up with Redis and Dragonfly
- **AND** Pester hits `/redis` and `/dragonfly`
- **THEN** each response shows Get, MGet, Incr, and Eval succeeded
- **AND** Eval used a Lua 5.1-safe script with keys in KEYS

### Requirement: Decoder fuzz never panics and never returns values on a dirty stream
The package SHALL include `FuzzReadReply` over the RESP decoder. That target MUST NOT panic. It MUST NOT return values together with a dirty stream. Seed corpus MUST be supplied via `f.Add` and MUST include well-formed status, integer, error, null bulk, empty bulk, bulk, arrays (including nested and null), huge length digits, truncated headers, unknown type bytes, and bulk lengths that do not fit in `int`. The package SHALL also include `FuzzParseLen` that MUST NOT accept a length whose `length+2` overflows. Tests MUST NOT commit a testdata fuzz corpus.

#### Scenario: Seed corpus runs as unit tests
- **WHEN** `go test` runs `FuzzReadReply` without `-fuzz`
- **THEN** each `f.Add` seed is executed
- **AND** none panic
- **AND** none return values while the stream is dirty

#### Scenario: ParseLen fuzz rejects overflowing length
- **WHEN** `FuzzParseLen` is given digit strings including values that would overflow `length+2`
- **THEN** it does not panic
- **AND** it does not report success for an overflowing length
