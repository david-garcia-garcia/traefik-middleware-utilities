# Spec Delta

## MODIFIED Requirements

### Requirement: Memory store uses Lua formulas
The in-memory store SHALL refill at `rate` tokens per second, cap at `burst`, consume 1, and compute wait in the same units Traefik's Redis script uses (tokens per microsecond, Unix microseconds, maxDelay microseconds). It MUST NOT import `golang.org/x/time/rate`. A missing key or a key whose effective ttl has elapsed SHALL start as a full bucket (`tokens = burst`, last equal to now) before consuming 1. Idle keys MUST NOT refill from elapsed since Unix epoch via `last = 0`. Entries SHALL expire after the effective ttl on later Allow (lazy). The effective ttl MUST NOT be shorter than the caller ttl. Construction SHALL fail when `rate <= 0`, rate is NaN, rate is infinite, `burst < 1`, `maxDelay < 0`, or the caller `ttl` is not a whole number of seconds of at least 1s. `NewMemory` and `NewRedis` SHALL share that construction gate.

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

#### Scenario: New rejects fractional ttl
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns the same ttl error as ttl below 1s
- **AND** no map entry is created

#### Scenario: New accepts whole-second ttl
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds

## ADDED Requirements

### Requirement: Idle TTL is at least the refill time
Constructors SHALL store one effective ttl for both stores: the caller ttl raised, in whole seconds, to ceil(burst divided by rate, plus maxDelay in seconds). The floor MUST be clamped to the largest whole-second duration that does not overflow. The effective ttl MUST NOT be shorter than the caller ttl. A caller ttl below 1s or with a fractional second SHALL still be rejected before that floor.

#### Scenario: Short ttl does not grant a fresh burst
- **WHEN** rate is 0.5 tokens per second, burst is 30, maxDelay is 0, and the caller ttl is 3s
- **AND** a key has consumed the burst
- **AND** 5 seconds pass on the limiter clock
- **THEN** the following admits are 2
- **AND** they are not 30

#### Scenario: Control ttl that already covers refill
- **WHEN** rate is 0.5 tokens per second, burst is 30, maxDelay is 0, and the caller ttl is 60s
- **AND** a key has consumed the burst
- **AND** 5 seconds pass on the limiter clock
- **THEN** the following admits are 2

#### Scenario: Caller ttl above the floor is kept
- **WHEN** rate is 0.5 tokens per second, burst is 30, maxDelay is 0, and the caller ttl is 120s
- **THEN** the idle lifetime is 120s
- **AND** it is not 60s

#### Scenario: maxDelay is inside the floor
- **WHEN** rate is 1, burst is 2, maxDelay is 1 hour, and the caller ttl is 2s
- **THEN** the idle lifetime is 3602s

#### Scenario: Enormous refill time is clamped
- **WHEN** burst divided by rate is larger than the largest whole-second duration
- **THEN** construction succeeds
- **AND** the idle lifetime is that largest whole-second duration
