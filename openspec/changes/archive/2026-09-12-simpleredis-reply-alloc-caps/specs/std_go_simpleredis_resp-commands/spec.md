## ADDED Requirements

### Requirement: Over-cap bulk or array header is redis:issue?
A `$` bulk length greater than `64 << 20` or a `*` array count greater than `1 << 20` SHALL return an error whose `Error()` text is `redis:issue?`. That connection MUST NOT re-enter the idle pool. The command MUST NOT retry that error. A truncated bulk or array whose announced size is at or under those ceilings SHALL still be I/O (`redis:unreachable` on EOF). An over-cap header with no payload MUST NOT take that truncated I/O path.

#### Scenario: Over-cap bulk Get is redis:issue?
- **WHEN** Get receives a `$` header whose length is greater than `64 << 20` and no payload
- **THEN** Get returns `redis:issue?`
- **AND** the idle pool is empty
- **AND** the error is not `redis:unreachable`

#### Scenario: Over-cap array is redis:issue?
- **WHEN** the peer replies with a `*` header whose count is greater than `1 << 20`
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: MGET-shaped over-cap bulk element is redis:issue?
- **WHEN** MGet receives an array whose `$` element length is greater than `64 << 20`
- **THEN** MGet returns `redis:issue?`
- **AND** the idle pool is empty
