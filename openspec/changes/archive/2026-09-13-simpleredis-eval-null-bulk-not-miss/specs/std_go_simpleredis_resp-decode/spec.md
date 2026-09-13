## MODIFIED Requirements

### Requirement: Lengths parse from header bytes
Bulk and array header lengths SHALL be parsed from the bytes after the type byte. An optional leading minus SHALL be accepted so `$-1` and `*-1` parse as negative lengths. A top-level `$` whose length is less than 0 SHALL return one nil slot and a nil error; that reply MUST NOT be `redis:miss`. Empty remainder or non-digit bytes SHALL return `redis:issue?`. The parse MUST NOT use `unsafe`. The parse MUST NOT import `go-redis`.

#### Scenario: Top-level null bulk is a nil slot
- **WHEN** the peer replies `$-1`
- **THEN** the decoder returns one nil slot
- **AND** the error is not `redis:miss`

#### Scenario: Garbage length is issue
- **WHEN** the array or bulk header length is empty or not digits
- **THEN** the command returns `redis:issue?`
