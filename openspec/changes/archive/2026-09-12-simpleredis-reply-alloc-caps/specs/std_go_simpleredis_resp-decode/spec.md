## ADDED Requirements

### Requirement: Bulk and array headers above package ceilings do not allocate
The decoder SHALL reject a bulk length greater than `64 << 20` bytes and an array element count greater than `1 << 20` after a successful length parse and after the bulk-miss check. Those ceilings MUST be package constants, not session `Config` fields. An over-ceiling bulk or array header SHALL return `redis:issue?` and MUST NOT allocate a payload buffer or an array of that announced size. Array `$` elements SHALL use the same bulk ceiling as a top-level bulk. Lengths at or under the bulk ceiling SHALL still allocate `length+2` bytes and read that many with a full read. `parseLen` overflow (a digit string that does not fit in `int`) SHALL remain `redis:issue?` without a wrap.

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

## MODIFIED Requirements

### Requirement: Escaping plus and integer payloads are copies
When a `+` or `:` payload is returned to the caller or stored in an array slot, the session SHALL copy those bytes before the next read on that connection and before the connection is released to the idle pool. Header-only uses (type byte, length parse) MUST NOT keep the `ReadSlice` view across a later read. A `-` error payload MAY rely on converting the message to `string` (that already copies). Bulk `$` payloads stay owned by `readBulk`'s allocated buffer. For a bulk length at or under the package bulk ceiling, `readBulk` SHALL keep `make([]byte, length+2)` plus `io.ReadFull`. A bulk length above that ceiling MUST NOT allocate that slice.

#### Scenario: Held payload survives a later command
- **WHEN** a command returns a `+` or `:` payload
- **AND** the caller holds that slice
- **AND** a second command on the same connection returns a distinct `+` or `:` payload
- **THEN** the first slice still equals the first payload
