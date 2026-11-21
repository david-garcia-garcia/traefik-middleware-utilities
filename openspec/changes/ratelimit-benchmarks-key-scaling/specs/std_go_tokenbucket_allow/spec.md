## ADDED Requirements

### Requirement: Memory store caps sources and removes expired ones first
The in-memory store SHALL hold at most 65536 sources. Before storing a new source (missing or TTL-expired key), it SHALL remove sources whose `ttl` has elapsed, and SHALL remove a live source only when the store is still at the cap with no elapsed source left. Which live source is removed is not specified. Removing sources MUST NOT change the admit, wait, or deny result of a source that stays stored.

#### Scenario: Full store drops expired sources before live ones
- **WHEN** the in-memory store holds 65536 sources and some of them have an elapsed `ttl` by the limiter clock
- **AND** Allow is called on a key that is not stored
- **THEN** the sources with an elapsed `ttl` are no longer stored
- **AND** every source whose `ttl` has not elapsed is still stored with its tokens unchanged
- **AND** the new key is admitted as a full bucket (`burst - 1` tokens remain)

#### Scenario: Full store of live sources stays at the cap
- **WHEN** the in-memory store holds 65536 sources and none has an elapsed `ttl`
- **AND** Allow is called on a key that is not stored
- **THEN** the store holds 65536 sources after that Allow
- **AND** the new key is admitted as a full bucket (`burst - 1` tokens remain)

#### Scenario: Removed source comes back as a full bucket
- **WHEN** a live source was removed to make room for a new key
- **AND** Allow is called again on the removed key
- **THEN** the store treats it as a full bucket (`tokens = burst`, last equal to now) before consuming 1
- **AND** that Allow returns allowed true
