## MODIFIED Requirements

### Requirement: Exact mode increments Redis on every Take
When `sync_rate` is zero, each Take SHALL increment the current-window key and SHALL set a TTL of two window lengths when that key has no TTL (`PTTL < 0`), including when the increment is not 1. Exact Take SHALL perform that increment and expire in one EVAL with the current-window key declared in `KEYS`. Exact Take MUST NOT refresh TTL when the key already has a TTL. Exact Take MUST NOT delete the key when expire would fail. Previous-window reads SHALL use `GET` (`redis:miss` counts as zero). Counter updates MUST NOT be a GET-then-SET of the integer.

#### Scenario: Exact mode expires the new window key
- **WHEN** `sync_rate` is 0
- **AND** Take creates a missing current-window key
- **THEN** Redis TTL on that key is two window lengths
- **WHEN** that TTL elapses
- **THEN** a later Take in a new window admits again

#### Scenario: Exact mode expires a leftover no-TTL key
- **WHEN** `sync_rate` is 0
- **AND** the current-window key already exists with count at least 1 and no TTL
- **AND** Take is called for that key
- **THEN** Redis TTL on that key is two window lengths
- **AND** the increment is not discarded
