## Purpose

Redis EVAL of Traefik's token-bucket script so in-process and Redis/Dragonfly Allow share one meaning: same rate, burst, maxDelay, and ttl, with keys declared for Dragonfly.

## Requirements

### Requirement: Eval runs the copied Traefik script
The Redis store SHALL admit via `simpleredis.Eval` of the copied Traefik `AllowTokenBucketRaw` script (Traefik Labs MIT notice kept). The hash key SHALL be listed in `KEYS`. The script SHALL use `#rl_source == 4` (not `table.maxn`). Go MUST NOT GET or SET that hash. v1 MUST NOT use EVALSHA. ARGV SHALL pass `rate/1e6`, burst, ttl seconds, Unix microseconds, and maxDelay microseconds.

#### Scenario: Dragonfly dense HGETALL length
- **WHEN** Allow runs the script on Dragonfly
- **AND** the hash has last and tokens
- **THEN** the script treats that hash as present (`#rl_source == 4`)
- **AND** MUST NOT call `table.maxn`

#### Scenario: Hash is not written from Go
- **WHEN** Allow updates last and tokens
- **THEN** those fields change only inside the Eval script
- **AND** Go MUST NOT issue GET or SET on that key for the bucket

### Requirement: Two instances share one key
Two limiter instances with two SimpleRedis clients SHALL share burst and wait for the same opaque Redis key. The second instance MUST NOT grant a second full burst while the first has already consumed it.

#### Scenario: No double burst
- **WHEN** two Redis limiters share one key and the same rate, burst, maxDelay, and ttl
- **AND** the first instance consumes the full burst after idle
- **THEN** the next Allow on the second instance is delayed or denied per maxDelay
- **AND** MUST NOT admit another full burst

### Requirement: Memory and Redis agree
For the same rate, burst, maxDelay, ttl, and Allow sequence (same test clock), memory and Redis SHALL return the same allowed boolean and the same wait class (zero, positive at most maxDelay, or greater than maxDelay).

#### Scenario: Same sequence admit and deny
- **WHEN** a memory limiter and a Redis limiter share rate, burst, maxDelay, and ttl
- **AND** the same Allow sequence runs on both with aligned clocks
- **THEN** each step's allowed value matches
- **AND** each step's wait is either both zero, both positive and at most maxDelay, or both greater than maxDelay

### Requirement: New rejects ttl Redis cannot expire in seconds
Construction SHALL fail when `ttl` is not a whole number of seconds. Memory expire lifetime and Redis EXPIRE seconds SHALL then be the same duration. The script MUST keep integer-second `EXPIRE`. Construction MUST NOT succeed for a fractional `ttl` while Memory expires at the full Duration and Redis EXPIRE uses truncated seconds.

#### Scenario: Fractional ttl never reaches Allow
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns an error
- **AND** no EVAL is sent

#### Scenario: Whole-second ttl still constructs
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds

### Requirement: Live Redis and Dragonfly
Live tests SHALL call Allow against each of Redis and Dragonfly whose address is set using `TOKENBUCKET_LIVE_REDIS` and `TOKENBUCKET_LIVE_DRAGONFLY`. Tests SHALL skip when both addrs are unset or under `-short`. When exactly one address is set they MUST run that engine and MUST NOT fail for the missing engine. CI `e2e-redis` SHALL start Redis, set `TOKENBUCKET_LIVE_REDIS`, and MUST NOT set `TOKENBUCKET_LIVE_DRAGONFLY`. CI `e2e-dragonfly` SHALL start Dragonfly, set `TOKENBUCKET_LIVE_DRAGONFLY`, and MUST NOT set `TOKENBUCKET_LIVE_REDIS`. The unit `test` job MUST pass `-short` and MUST NOT start those engines. Yaegi live SHALL run the same scenarios; the compiled test owns start/skip; the interpreted probe calls Allow. Live scenarios SHALL include burst after idle, two-instance share, memory/Redis agreement, and refund when wait exceeds maxDelay.

#### Scenario: Redis job table-driven
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** Allow scenarios run against Redis
- **AND** the live tests MUST NOT skip

#### Scenario: Dragonfly job table-driven
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** Allow scenarios run against Dragonfly
- **AND** the live tests MUST NOT skip

#### Scenario: Redis job proves refund past maxDelay
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** Allow after a wait greater than maxDelay does not admit a stacked consume against Redis

#### Scenario: Dragonfly job proves refund past maxDelay
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** Allow after a wait greater than maxDelay does not admit a stacked consume against Dragonfly
