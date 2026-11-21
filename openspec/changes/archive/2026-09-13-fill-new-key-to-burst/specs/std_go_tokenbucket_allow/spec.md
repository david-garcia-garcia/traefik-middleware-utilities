## MODIFIED Requirements

### Requirement: Memory store uses Lua formulas
The in-memory store SHALL refill at `rate` tokens per second, cap at `burst`, consume 1, and compute wait in the same units Traefik's Redis script uses (tokens per microsecond, Unix microseconds, maxDelay microseconds). It MUST NOT import `golang.org/x/time/rate`. A missing key or a key whose `ttl` has elapsed SHALL start as a full bucket (`tokens = burst`, last equal to now) before consuming 1. Idle keys MUST NOT refill from elapsed since Unix epoch via `last = 0`. Entries SHALL expire after the caller `ttl` on later Allow (lazy). Construction SHALL fail when `rate <= 0`, `burst < 1`, `maxDelay < 0`, or `ttl < 1s`.

#### Scenario: New rejects invalid clock
- **WHEN** NewMemory is called with rate 0 or a negative rate
- **THEN** construction returns an error
- **AND** no map entry is created

#### Scenario: Idle fills to burst
- **WHEN** a new key has no prior tokens
- **AND** Allow is called
- **THEN** the store treats the bucket as filled to `burst` before consuming 1
- **AND** that Allow returns allowed true when `burst >= 1`

#### Scenario: New key at Unix epoch fills to burst
- **WHEN** the test clock is Unix epoch (`UnixMicro` 0)
- **AND** burst is 5 and rate is 1
- **AND** Allow is called on a new key
- **THEN** the store treats the bucket as filled to 5 before consuming 1
- **AND** remaining tokens after that Allow are at least 4
- **AND** that Allow MUST NOT leave tokens at -1 from elapsed 0

#### Scenario: New key with burst larger than elapsed times rate fills to burst
- **WHEN** the test clock is `Unix(1_700_000_000, 0)`
- **AND** burst is `1e12` and rate is 1
- **AND** Allow is called on a new key
- **THEN** remaining tokens after that Allow are at least `burst - 1`
- **AND** remaining tokens MUST NOT be only elapsed-from-epoch times rate minus 1
