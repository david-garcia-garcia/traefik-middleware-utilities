# Spec Delta

## MODIFIED Requirements

### Requirement: Eval runs the copied Traefik script
The Redis store SHALL admit via `simpleredis.Eval` of the Traefik `AllowTokenBucketRaw` script with Traefik Labs MIT notice kept, plus this product's empty-hash fill and last-field persist: when `HGETALL` is not four fields (`#rl_source ~= 4`), the script SHALL set tokens to burst and last to now (elapsed 0) before the existing refill and consume; persisted `last` SHALL be the later of the previous hash `last` and ARGV now `t`. Elapsed SHALL still treat `t` behind `last` as zero elapsed. The hash key SHALL be listed in `KEYS`. The script SHALL use `#rl_source == 4` (not `table.maxn`). Go MUST NOT GET or SET that hash. v1 MUST NOT use EVALSHA. ARGV SHALL pass `rate/1e6`, burst, effective ttl seconds, Unix microseconds, and maxDelay microseconds. Those ttl seconds SHALL be the same idle lifetime the in-memory store uses and MUST NOT be fewer than the caller ttl.

#### Scenario: Dragonfly dense HGETALL length
- **WHEN** Allow runs the script on Dragonfly
- **AND** the hash has last and tokens
- **THEN** the script treats that hash as present (`#rl_source == 4`)
- **AND** MUST NOT call `table.maxn`

#### Scenario: Hash is not written from Go
- **WHEN** Allow updates last and tokens
- **THEN** those fields change only inside the Eval script
- **AND** Go MUST NOT issue GET or SET on that key for the bucket

#### Scenario: Empty hash fills to burst then consume 1
- **WHEN** the hash is missing or `HGETALL` is not four fields
- **AND** Allow is called
- **THEN** the script treats tokens as burst and last as now before consume 1
- **AND** remaining tokens after that Allow are at least `burst - 1` when burst is at least 1
- **AND** that fill MUST NOT depend on last 0 plus elapsed since Unix epoch

#### Scenario: HSET last is persist-max
- **WHEN** the script has a previous hash last and ARGV `t` is earlier than that last
- **THEN** elapsed is clamped so refill is not negative
- **AND** HSET last is the previous last, not raw `t`

#### Scenario: Short caller ttl is raised in ARGV
- **WHEN** NewRedis is called with rate 1, burst 2, maxDelay 1 microsecond, and caller ttl 2s
- **AND** Allow runs
- **THEN** the ttl seconds argument is 3

#### Scenario: Caller ttl above the floor is the ARGV
- **WHEN** NewRedis is called with rate 1, burst 2, maxDelay 0, and caller ttl 120s
- **AND** Allow runs
- **THEN** the ttl seconds argument is 120

### Requirement: New rejects ttl Redis cannot expire in seconds
Construction SHALL fail when the caller `ttl` is not a whole number of seconds. Memory expire lifetime and Redis EXPIRE seconds SHALL then be the same effective duration. That duration MUST NOT be shorter than the caller ttl. The script MUST keep integer-second `EXPIRE`. Construction MUST NOT succeed for a fractional caller `ttl` while Memory expires at the full Duration and Redis EXPIRE uses truncated seconds.

#### Scenario: Fractional ttl never reaches Allow
- **WHEN** NewMemory or NewRedis is called with ttl 1500ms
- **THEN** construction returns an error
- **AND** no EVAL is sent

#### Scenario: Whole-second ttl still constructs
- **WHEN** NewMemory or NewRedis is called with ttl 2s
- **THEN** construction succeeds
