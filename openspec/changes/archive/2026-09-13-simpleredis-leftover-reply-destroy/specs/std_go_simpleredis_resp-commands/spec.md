## MODIFIED Requirements

### Requirement: Get returns the stored value or a miss
`Get(name)` SHALL send Redis `GET` for that key. A bulk reply SHALL return those bytes. After a one-slot reply, a nil slot (RESP2 null bulk `$-1`) SHALL return an error whose `Error()` text is `redis:miss`. Empty bulk `$0` SHALL return a non-nil empty slice and MUST NOT return `redis:miss`. After a peer writes an extra well-formed reply on the same connection, Get MUST return the value for its own key or an error and MUST NOT return another key’s bytes.

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

#### Scenario: Empty bulk is not a miss
- **WHEN** Get receives empty bulk `$0`
- **THEN** Get returns a non-nil empty slice
- **AND** the error is not `redis:miss`

#### Scenario: Get after stray extra is own-key or error
- **WHEN** `PoolSize` is `1` and `MaxRetries` is `-1`
- **AND** a peer answers `GET kN` with `vN` and appends one extra well-formed bulk every 5th command
- **AND** Get is called for `k0` through at least `k11`
- **THEN** each Get returns the bytes for that key or an error
- **AND** no Get returns another key’s bytes or the stray bulk as a success
