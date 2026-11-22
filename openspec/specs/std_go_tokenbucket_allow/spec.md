## Purpose

Token-bucket Allow: admit or delay one consume on an opaque key against refill rate, burst, and maxDelay, using the Traefik Lua formulas in process. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Allow maps admit, wait, and deny
`Allow(ctx, key)` SHALL consume one token for the opaque `key` and return a `Status` and an error. `Memory.Allow` and `Redis.Allow` SHALL take the same arguments and return the same `Status` type. A caller with no deadline SHALL pass `context.Background()`. The wait is the time until tokens reach zero after this consume, in microseconds; `Status` SHALL report it as `Delay` when allowed and as `RetryAfter` when denied (`std_go_tokenbucket_status`). `Allowed` SHALL be false with `Delay` and `RetryAfter` zero when the store cannot reserve (burst cannot cover this consume). `Allowed` SHALL be false when the wait in microseconds is greater than `maxDelay` truncated to whole microseconds (the store SHALL refund that consume). `Allowed` SHALL be true when that wait in microseconds is at most `maxDelay` truncated to whole microseconds. The `Delay` and `RetryAfter` durations MUST NOT decide `Allowed`. When the error is non-nil, the `Status` SHALL be the zero value. The library MUST NOT sleep. Prefixing of `key` is the caller's job.

#### Scenario: Burst after idle then next delayed or denied
- **WHEN** a key has been idle long enough to fill to `burst`
- **AND** Allow is called `burst` times for that key with `maxDelay` large enough to admit those consumes
- **THEN** each of those Allows returns `Allowed` true
- **WHEN** Allow is called once more for that key
- **THEN** that Allow returns `Allowed` false with a positive `RetryAfter` if the wait in microseconds is greater than `maxDelay` truncated to whole microseconds, or `Allowed` true with a positive `Delay` if that wait in microseconds is at most `maxDelay` truncated to whole microseconds

#### Scenario: Refund when wait exceeds maxDelay
- **WHEN** tokens after consume are negative and the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **THEN** Allow refunds that consume
- **AND** returns `Allowed` false
- **AND** the next Allow on that key sees the refunded tokens (not a second consume stacked on the denied one)

#### Scenario: Refund and admit share microseconds
- **WHEN** the wait in microseconds is greater than `maxDelay` truncated to whole microseconds
- **AND** that wait converted to a duration is still at most `maxDelay`
- **THEN** Allow refunds that consume
- **AND** returns `Allowed` false
- **AND** a following Allow on that key at the same instant is not a stacked consume

#### Scenario: Caller owns the key
- **WHEN** Allow is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

#### Scenario: Error returns the zero status
- **WHEN** Allow returns a non-nil error (cancelled context, Redis error, or a malformed Eval reply)
- **THEN** the returned `Status` is the zero value
- **AND** its `Allowed` is false

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

### Requirement: Memory store caps sources and removes expired ones first
The in-memory store SHALL hold at most 65536 sources. Before storing a new source (missing or TTL-expired key), it SHALL remove sources whose `ttl` has elapsed, and SHALL remove a live source only when the store is still at the cap with no elapsed source left. That ordering holds while the limiter clock does not move backwards; a test clock that moves backwards MAY leave an elapsed source stored while a live one is removed at the cap. Which live source is removed is not specified. Removing sources MUST NOT change the admit, wait, or deny result of a source that stays stored.

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

#### Scenario: Idle expired sources leave one stored source
- **WHEN** the in-memory store holds ten sources whose `ttl` has elapsed by the limiter clock
- **AND** Allow is called on a key that is not stored
- **THEN** the store holds exactly one source
- **AND** that source is the new key

### Requirement: Eval tokens that is not a finite number is not a consume
When the Redis store is used and Eval returns three fields, Allow SHALL read the third field as the tokens left after this Allow. A tokens field that is not a finite number, including `nan`, `+Inf`, `-Inf`, `inf`, or text that does not parse, SHALL return `tokenbucket: eval tokens is not a number`. Allow MUST NOT return a `Status` with a nil error in that case. The Eval script MUST NOT change to produce this field.

#### Scenario: Non-finite tokens is tokens-is-not-a-number
- **WHEN** Eval returns three fields whose wait is a finite number and whose tokens is `nan`, `+Inf`, `-Inf`, `inf`, or `abc`
- **THEN** Allow returns `tokenbucket: eval tokens is not a number`
- **AND** the returned `Status` is the zero value

#### Scenario: Tokens text from either engine parses
- **WHEN** Eval returns a tokens field written as `4`, `4.0`, `-1.0`, `0.3`, or `1e+15`
- **THEN** Allow returns a nil error
- **AND** the `Status` is built from that tokens value

### Requirement: Concurrent first requests share one in-process bucket
The in-process store SHALL hold the mutex that guards the bucket map across lookup, create, and consume of one Allow, so concurrent first Allows on a new opaque key share one bucket.

#### Scenario: Concurrent first requests admit at most burst
- **WHEN** 16 goroutines call Allow at the same instant on a new opaque key
- **AND** rate is 1 per minute, burst is 5, and maxDelay is 500 ms
- **THEN** at most 5 of those Allows return `Allowed` true
- **AND** the store MUST NOT create a second full bucket for that key
- **AND** a non-nil Allow error fails the check

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

### Requirement: Memory key space is the instance
Each `*Memory` SHALL be its own key space. `Allow` on one value MUST NOT consume tokens stored on a different `*Memory`. Two values that receive the same opaque `key` SHALL each admit up to `burst`. The library MUST NOT keep a process-wide Memory map or key Memory by middleware name.

#### Scenario: Shared Memory admits burst in total
- **WHEN** two callers share one `*Memory`
- **AND** they `Allow` the same opaque `key` with `maxDelay` 0 until that store denies
- **THEN** the number of `Allowed` true results across both callers is `burst`

#### Scenario: Separate Memory instances admit twice burst
- **WHEN** two `*Memory` values are constructed separately
- **AND** each `Allow`s the same opaque `key` with `maxDelay` 0 until that value denies
- **THEN** the number of `Allowed` true results across both values is `2 * burst`

### Requirement: Redis instances share one key
When the Redis store is used, bucket state lives on the server. Any `*Redis` that `Allow`s the same opaque `key` SHALL share that budget and admit `burst` in total across those instances.

#### Scenario: Two Redis limiters on one key admit burst in total
- **WHEN** two `*Redis` values `Allow` the same opaque `key` on a live engine with `maxDelay` 0 until that key denies
- **THEN** the number of `Allowed` true results across both values is `burst`
