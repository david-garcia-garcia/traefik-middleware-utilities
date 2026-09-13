## MODIFIED Requirements

### Requirement: Memory store uses Lua formulas
The in-memory store SHALL refill at `rate` tokens per second, cap at `burst`, consume 1, and compute wait in the same units Traefik's Redis script uses (tokens per microsecond, Unix microseconds, maxDelay microseconds). It MUST NOT import `golang.org/x/time/rate`. Idle keys SHALL accumulate up to `burst`. Entries SHALL expire after the caller `ttl` on later Allow (lazy). Construction SHALL fail when `rate <= 0`, `burst < 1`, `maxDelay < 0`, or `ttl` is not a whole number of seconds of at least 1s.

#### Scenario: New rejects invalid clock
- **WHEN** NewMemory is called with rate 0 or a negative rate
- **THEN** construction returns an error
- **AND** no map entry is created

#### Scenario: Idle fills to burst
- **WHEN** a new key has no prior tokens
- **AND** Allow is called
- **THEN** the store treats the bucket as filled to `burst` before consuming 1
- **AND** that Allow returns allowed true when `burst >= 1`

#### Scenario: New rejects fractional ttl
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns the same ttl error as ttl below 1s
- **AND** no map entry is created

#### Scenario: New accepts whole-second ttl
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds
