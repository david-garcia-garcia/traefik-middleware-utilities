## ADDED Requirements

### Requirement: New rejects ttl Redis cannot expire in seconds
Construction SHALL fail when `ttl` is not a whole number of seconds. Memory expire lifetime and Redis EXPIRE seconds SHALL then be the same duration. The script MUST keep integer-second `EXPIRE`. Construction MUST NOT succeed for a fractional `ttl` while Memory expires at the full Duration and Redis EXPIRE uses truncated seconds.

#### Scenario: Fractional ttl never reaches Allow
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns an error
- **AND** no EVAL is sent

#### Scenario: Whole-second ttl still constructs
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds
