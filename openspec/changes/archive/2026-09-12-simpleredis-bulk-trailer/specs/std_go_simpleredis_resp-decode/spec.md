## MODIFIED Requirements

### Requirement: Escaping plus and integer payloads are copies
When a `+` or `:` payload is returned to the caller or stored in an array slot, the session SHALL copy those bytes before the next read on that connection and before the connection is released to the idle pool. Header-only uses (type byte, length parse) MUST NOT keep the `ReadSlice` view across a later read. A `-` error payload MAY rely on converting the message to `string` (that already copies). Bulk `$` payloads stay owned by `readBulk`'s allocated buffer. `readBulk` SHALL keep `make([]byte, length+2)` plus `io.ReadFull`. After a complete `io.ReadFull` of `length+2`, the last two bytes SHALL be CR then LF; otherwise the session SHALL return `redis:issue?`. Empty bulk `$0` whose trailer is CRLF SHALL succeed.

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
