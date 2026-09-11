## ADDED Requirements

### Requirement: Idle encode scratch is not retained past 64 KiB
When a reusable connection is returned to the idle pool, if the encode scratch capacity exceeds 64 KiB, the session SHALL drop that scratch so the next command reallocates. A connection whose scratch capacity is at most 64 KiB SHALL keep it.

#### Scenario: Large SET then idle reuse trims scratch
- **WHEN** a command encodes a value that grows the encode scratch past 64 KiB
- **AND** that connection is released as reusable
- **THEN** the idle connection’s encode scratch capacity is at most 64 KiB
- **WHEN** a later command reuses that connection
- **THEN** encode still succeeds
