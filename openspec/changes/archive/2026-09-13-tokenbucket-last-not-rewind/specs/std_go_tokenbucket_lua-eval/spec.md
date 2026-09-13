## MODIFIED Requirements

### Requirement: Eval runs the copied Traefik script
The Redis store SHALL admit via `simpleredis.Eval` of Traefik `AllowTokenBucketRaw` (Traefik Labs MIT notice kept) with one intentional last-field change: persisted `last` SHALL be the later of the previous hash `last` and ARGV now `t`. Elapsed SHALL still treat `t` behind `last` as zero elapsed. The hash key SHALL be listed in `KEYS`. The script SHALL use `#rl_source == 4` (not `table.maxn`). Go MUST NOT GET or SET that hash. v1 MUST NOT use EVALSHA. ARGV SHALL pass `rate/1e6`, burst, ttl seconds, Unix microseconds, and maxDelay microseconds.

#### Scenario: Dragonfly dense HGETALL length
- **WHEN** Allow runs the script on Dragonfly
- **AND** the hash has last and tokens
- **THEN** the script treats that hash as present (`#rl_source == 4`)
- **AND** MUST NOT call `table.maxn`

#### Scenario: Hash is not written from Go
- **WHEN** Allow updates last and tokens
- **THEN** those fields change only inside the Eval script
- **AND** Go MUST NOT issue GET or SET on that key for the bucket

#### Scenario: HSET last is persist-max
- **WHEN** the script has a previous hash last and ARGV `t` is earlier than that last
- **THEN** elapsed is clamped so refill is not negative
- **AND** HSET last is the previous last, not raw `t`

## ADDED Requirements

### Requirement: Script last does not rewind
The Eval script SHALL persist `last` as the later of the previous bucket `last` and ARGV `t`. Tests SHALL assert the script text does not HSET last as raw `t` after the elapsed clamp.

#### Scenario: Script text persists max last
- **WHEN** unit tests inspect the Eval script
- **THEN** HSET last is the persist-max of previous last and `t`
- **AND** MUST NOT write raw `t` as last when `t` is behind previous last
