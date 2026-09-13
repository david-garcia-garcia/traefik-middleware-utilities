## Purpose

Token-bucket Allow: admit or delay one consume on an opaque key against refill rate, burst, and maxDelay, using the Traefik Lua formulas in process. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Allow maps admit, wait, and deny
`Allow(ctx, key)` SHALL consume one token for the opaque `key` and return `allowed` plus a wait duration. `Memory.Allow` and `Redis.Allow` SHALL take the same arguments. A caller with no deadline SHALL pass `context.Background()`. `allowed` SHALL be false and wait zero when the store cannot reserve (burst cannot cover this consume). `allowed` SHALL be false when the wait until tokens reach zero, in microseconds, is greater than `maxDelay` truncated to whole microseconds (the store SHALL refund that consume). `allowed` SHALL be true when that wait in microseconds is at most `maxDelay` truncated to whole microseconds. The wait duration return MUST NOT decide `allowed`. The library MUST NOT sleep. Prefixing of `key` is the caller's job.

#### Scenario: Burst after idle then next delayed or denied
- **WHEN** a key has been idle long enough to fill to `burst`
- **AND** Allow is called `burst` times for that key with `maxDelay` large enough to admit those consumes
- **THEN** each of those Allows returns allowed true
- **WHEN** Allow is called once more for that key
- **THEN** that Allow returns allowed false if the wait in microseconds is greater than `maxDelay` truncated to whole microseconds, or allowed true with a positive wait if that wait in microseconds is at most `maxDelay` truncated to whole microseconds

#### Scenario: Refund when wait exceeds maxDelay
- **WHEN** tokens after consume are negative and the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **THEN** Allow refunds that consume
- **AND** returns allowed false
- **AND** the next Allow on that key sees the refunded tokens (not a second consume stacked on the denied one)

#### Scenario: Refund and admit share microseconds
- **WHEN** the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **AND** that wait converted to a duration is still at most `maxDelay`
- **THEN** Allow refunds that consume
- **AND** returns allowed false
- **AND** a following Allow on that key at the same instant is not a stacked consume

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Memory store uses Lua formulas
The in-memory store SHALL refill at `rate` tokens per second, cap at `burst`, consume 1, and compute wait in the same units Traefik's Redis script uses (tokens per microsecond, Unix microseconds, maxDelay microseconds). It MUST NOT import `golang.org/x/time/rate`. Idle keys SHALL accumulate up to `burst`. Entries SHALL expire after the caller `ttl` on later Allow (lazy). Construction SHALL fail when `rate <= 0`, rate is NaN, rate is infinite, `burst < 1`, `maxDelay < 0`, or `ttl` is not a whole number of seconds of at least 1s. `NewMemory` and `NewRedis` SHALL share that construction gate.

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

#### Scenario: New rejects fractional ttl
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns the same ttl error as ttl below 1s
- **AND** no map entry is created

#### Scenario: New accepts whole-second ttl
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds

### Requirement: Persisted last does not rewind
The in-process store SHALL persist `last` as the later of the previous stored `last` and this consume's now (Unix microseconds). Elapsed for refill SHALL still treat a now behind `last` as zero elapsed so refill is never negative. The store MUST NOT persist a `last` earlier than the previous stored `last`. The in-process store SHALL read now after it holds the mutex that guards the bucket map, so lock order is clock order. Tests SHALL fail on dest for a sequential backward now and for a stale now sampled before a newer consume finishes, then pass after this change.

#### Scenario: Sequential backward now does not refill twice
- **WHEN** a key is admitted at t0 then at t2 (burst 1, maxDelay 0, rate 1)
- **AND** Allow is called with now at t1 (between t0 and t2)
- **THEN** that Allow is denied
- **AND** the stored `last` stays at t2
- **WHEN** Allow is called again with now at t2
- **THEN** that Allow is denied
- **AND** MUST NOT admit by refilling the t2-minus-t1 interval already granted

#### Scenario: Stale now sampled before lock does not rewind last
- **WHEN** two Allows overlap on one key: the first samples t1 and waits outside the mutex, the second samples t2 and runs to completion first
- **THEN** the first MUST NOT store `last` as t1 after the second stored t2
- **WHEN** Allow is called later with now at t2
- **THEN** that Allow is denied

### Requirement: Redis errors do not become deny
When the Redis store is used and Redis is unreachable or times out, Allow SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or denyOnError inside Allow.

#### Scenario: Unreachable Redis
- **WHEN** Allow cannot complete the script because the server is unreachable
- **THEN** Allow returns `redis:unreachable`
- **AND** does not return allowed false as a substitute

### Requirement: Eval wait that is not a finite number is not a consume
When the Redis store is used and Eval returns three fields, Allow SHALL treat the wait field as a finite number of microseconds. A wait that is not a number, including `nan`, `+Inf`, `-Inf`, and `inf`, SHALL return the existing wait-is-not-a-number error (`tokenbucket: eval wait is not a number`). Allow MUST NOT return allowed true with a nil error. Allow MUST NOT return allowed false with a nil error. Allow MUST NOT introduce a second error for that wait. Admit and refund comparisons MUST NOT replace this rule: a non-finite wait is not a delay that can be compared to maxDelay.

#### Scenario: Non-finite wait is wait-is-not-a-number
- **WHEN** Eval returns three fields whose wait is `nan`, `+Inf`, `-Inf`, or `inf`
- **THEN** Allow returns `tokenbucket: eval wait is not a number`
- **AND** does not return allowed true
- **AND** does not return a nil error

### Requirement: Unit and Yaegi prove Allow
Compiled tests SHALL prove burst-after-idle and refund on the memory store, and Eval encoding on a fake TCP Redis, without Traefik. Interpreted tests SHALL run the same Allow scenarios with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time.

#### Scenario: Yaegi Allow on fake Redis
- **WHEN** the Yaegi GOPATH interp loads non-test `tokenbucket` and `simpleredis` sources
- **AND** the probe calls Allow against the compiled fake
- **THEN** the probe result is success
- **AND** the interp MUST NOT enable unsafe
