## MODIFIED Requirements

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

### Requirement: Copy-on-escape and ErrBufferFull have compiled unit tests
Compiled tests in `simpleredis/` SHALL prove a held `+` or `:` slice stays correct after a later read on the same connection, using distinct payloads. Those tests MUST NOT use identical EVAL `:0` replies that hide aliasing. Compiled tests SHALL also prove a status or error line longer than 4096 bytes returns `redis:issue?` and is not pooled, and that an unterminated stream that fills the 4096-byte buffer returns `redis:issue?` without consuming more than 4096 bytes from the peer. Those tests MUST NOT require live Redis or Dragonfly. Those tests MUST NOT use `runtime.ReadMemStats`.

#### Scenario: Unit tests cover aliasing and long lines
- **WHEN** `go test ./simpleredis/...` runs
- **THEN** a test fails if a held `+` or `:` slice is overwritten by the next read
- **AND** a test fails if a line longer than 4096 bytes is decoded
- **AND** a test fails if an unterminated stream that fills the buffer is decoded or if more than 4096 bytes were consumed from the peer
