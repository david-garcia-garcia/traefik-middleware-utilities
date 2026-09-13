## ADDED Requirements

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

## MODIFIED Requirements

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
