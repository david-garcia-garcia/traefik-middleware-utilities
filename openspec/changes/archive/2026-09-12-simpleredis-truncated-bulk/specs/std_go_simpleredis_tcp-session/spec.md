## ADDED Requirements

### Requirement: Dirty reply is not returned to the idle pool
When a command’s reply is a short bulk read (the peer announces more payload bytes than it writes before closing), a malformed bulk length, or an array element whose head is not `$`, `:`, or `+`, the session SHALL NOT return that socket to the idle pool. A short bulk read SHALL return an error whose `Error()` text is `redis:unreachable` and MUST NOT return `redis:issue?`. A malformed bulk length (`$` followed by a non-integer) or an illegal array-element head SHALL return `redis:issue?`. After that failed command, a later command on the same client SHALL return the value for its own key; the discarded socket MUST NOT leak a prior payload. Truncated-payload coverage MUST be a unit test against a peer that can write raw bytes and close mid-stream; it MUST NOT depend on live Redis or Dragonfly emitting a lying length.

#### Scenario: Truncated bulk is unreachable and not pooled
- **WHEN** a Get receives `$100\r\n`, then 40 bytes, then a close
- **THEN** the command returns `redis:unreachable`
- **AND** the error is not `redis:issue?`
- **AND** the idle pool is empty after that call

#### Scenario: Second Get after truncate returns its own value
- **WHEN** that truncated Get has returned
- **AND** a later Get is issued for a key whose next reply is a complete bulk of known bytes
- **THEN** that Get returns those bytes
- **AND** the idle pool was empty after the truncated call

#### Scenario: Malformed bulk header is issue and not pooled
- **WHEN** a Get receives `$abc\r\n`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty after that call

#### Scenario: Illegal array-element head is issue and not pooled
- **WHEN** a command receives an array whose element head is neither `$`, `:`, nor `+`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty after that call
