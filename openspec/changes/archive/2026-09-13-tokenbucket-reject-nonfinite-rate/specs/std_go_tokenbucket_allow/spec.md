## MODIFIED Requirements

### Requirement: Memory store uses Lua formulas
The in-memory store SHALL refill at `rate` tokens per second, cap at `burst`, consume 1, and compute wait in the same units Traefik's Redis script uses (tokens per microsecond, Unix microseconds, maxDelay microseconds). It MUST NOT import `golang.org/x/time/rate`. Idle keys SHALL accumulate up to `burst`. Entries SHALL expire after the caller `ttl` on later Allow (lazy). Construction SHALL fail when `rate <= 0`, rate is NaN, rate is infinite, `burst < 1`, `maxDelay < 0`, or `ttl < 1s`. `NewMemory` and `NewRedis` SHALL share that construction gate.

#### Scenario: New rejects invalid clock
- **WHEN** NewMemory is called with rate 0 or a negative rate
- **THEN** construction returns an error
- **AND** no map entry is created

#### Scenario: New rejects non-finite rate
- **WHEN** NewMemory or NewRedis is called with a NaN rate or an infinite rate
- **THEN** construction returns an error
- **AND** no limiter is created

#### Scenario: Idle fills to burst
- **WHEN** a new key has no prior tokens
- **AND** Allow is called
- **THEN** the store treats the bucket as filled to `burst` before consuming 1
- **AND** that Allow returns allowed true when `burst >= 1`
