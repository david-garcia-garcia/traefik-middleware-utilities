## MODIFIED Requirements

### Requirement: Malformed RESP is a protocol issue and is not pooled
A reply whose type byte is not `+`, `-`, `:`, `$`, or `*` (including an HTTP-shaped first line), a line that does not end in CR before LF, an empty line, an unparseable `*` count, an `*` count less than 0, a truncated array element or bulk, an array element whose type is not `$`, `:`, or `+`, or a complete bulk payload whose two trailer bytes are not CR then LF SHALL return an error whose `Error()` text is `redis:issue?`, or an I/O error (`redis:unreachable` on EOF, `redis:timeout` on deadline). That connection MUST NOT re-enter the idle pool. A wrong bulk trailer MUST return `redis:issue?` and MUST NOT return `redis:unreachable`.

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

#### Scenario: Truncated bulk after a complete ReadSlice head returns own-value on the next Get
- **WHEN** `MaxRetries` is `-1`
- **AND** a Get receives `$100\r\n`, then 40 bytes, then a close
- **THEN** that Get returns `redis:unreachable`
- **AND** the idle pool is empty
- **WHEN** a later Get receives a complete bulk of known bytes
- **THEN** that Get returns those bytes

#### Scenario: Nested array is not pooled
- **WHEN** an array element is a nested array
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Retry after a dirty reused connection cannot dial
- **WHEN** a reused idle connection is dirtied by a truncated reply
- **AND** the retry dial fails
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Wrong bulk trailer is issue and not pooled
- **WHEN** a Get receives a complete `$` payload whose two trailer bytes are not CR then LF
- **THEN** the command returns `redis:issue?`
- **AND** the error is not `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Second Get after a wrong bulk trailer returns its own value
- **WHEN** `MaxRetries` is `-1`
- **AND** `PoolSize` is `1`
- **AND** a Get receives a complete bulk whose trailer is not CRLF and leftover bytes remain on that connection
- **THEN** that Get returns `redis:issue?`
- **AND** the idle pool is empty
- **WHEN** a later Get receives a complete bulk of known bytes
- **THEN** that Get returns those bytes
- **AND** those bytes are not remnants of the first payload
