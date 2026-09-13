## Purpose

Token-bucket Allow: admit or delay one consume on an opaque key against refill rate, burst, and maxDelay, using the Traefik Lua formulas in process. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Allow maps admit, wait, and deny
`Allow(ctx, key)` SHALL consume one token for the opaque `key` and return `allowed` plus a wait duration. `Memory.Allow` and `Redis.Allow` SHALL take the same arguments. A caller with no deadline SHALL pass `context.Background()`. `allowed` SHALL be false and wait zero when the store cannot reserve (burst cannot cover this consume). `allowed` SHALL be false and wait greater than `maxDelay` when the consume would wait longer than `maxDelay` (the store SHALL refund that consume). `allowed` SHALL be true when the consume is admitted now or after waiting at most `maxDelay`. The library MUST NOT sleep. Prefixing of `key` is the caller's job.

#### Scenario: Burst after idle then next delayed or denied
- **WHEN** a key has been idle long enough to fill to `burst`
- **AND** Allow is called `burst` times for that key with `maxDelay` large enough to admit those consumes
- **THEN** each of those Allows returns allowed true
- **WHEN** Allow is called once more for that key
- **THEN** that Allow returns allowed false if the computed wait is greater than `maxDelay`, or allowed true with a positive wait if that wait is at most `maxDelay`

#### Scenario: Refund when wait exceeds maxDelay
- **WHEN** tokens after consume are negative and `-tokens / rate` is greater than `maxDelay`
- **THEN** Allow refunds that consume
- **AND** returns allowed false
- **AND** the next Allow on that key sees the refunded tokens (not a second consume stacked on the denied one)

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

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

### Requirement: Redis errors do not become deny
When the Redis store is used and Redis is unreachable or times out, Allow SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or denyOnError inside Allow.

#### Scenario: Unreachable Redis
- **WHEN** Allow cannot complete the script because the server is unreachable
- **THEN** Allow returns `redis:unreachable`
- **AND** does not return allowed false as a substitute

### Requirement: Unit and Yaegi prove Allow
Compiled tests SHALL prove burst-after-idle and refund on the memory store, and Eval encoding on a fake TCP Redis, without Traefik. Interpreted tests SHALL run the same Allow scenarios with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time.

#### Scenario: Yaegi Allow on fake Redis
- **WHEN** the Yaegi GOPATH interp loads non-test `tokenbucket` and `simpleredis` sources
- **AND** the probe calls Allow against the compiled fake
- **THEN** the probe result is success
- **AND** the interp MUST NOT enable unsafe
