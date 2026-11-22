# Spec Delta

## MODIFIED Requirements

### Requirement: Eval runs the copied Traefik script
The Redis store SHALL admit via `simpleredis.Eval` of the Traefik `AllowTokenBucketRaw` script with Traefik Labs MIT notice kept, plus this product's empty-hash fill and last-field persist: when `HMGET` of `last` and `tokens` is not both truthy, the script SHALL set tokens to burst and last to now (elapsed 0) before the existing refill and consume; persisted `last` SHALL be the later of the previous hash `last` and ARGV now `t`. Elapsed SHALL still treat `t` behind `last` as zero elapsed. The hash key SHALL be listed in `KEYS`. The script SHALL read `last` and `tokens` by name with `HMGET` (not `HGETALL` by position, and not `table.maxn`). Go MUST NOT GET or SET that hash. v1 MUST NOT use EVALSHA. ARGV SHALL pass `rate/1e6`, burst, ttl seconds, Unix microseconds, and maxDelay microseconds.

#### Scenario: Dragonfly dense HGETALL length
- **WHEN** Allow runs the script on Dragonfly
- **AND** the hash has last and tokens
- **THEN** the script treats that hash as present from `HMGET` of those names
- **AND** MUST NOT call `table.maxn`
- **AND** MUST NOT assign last or tokens from `HGETALL` position

#### Scenario: Hash is not written from Go
- **WHEN** Allow updates last and tokens
- **THEN** those fields change only inside the Eval script
- **AND** Go MUST NOT issue GET or SET on that key for the bucket

#### Scenario: Empty hash fills to burst then consume 1
- **WHEN** the hash is missing or either `HMGET` field is not truthy
- **AND** Allow is called
- **THEN** the script treats tokens as burst and last as now before consume 1
- **AND** remaining tokens after that Allow are at least `burst - 1` when burst is at least 1
- **AND** that fill MUST NOT depend on last 0 plus elapsed since Unix epoch

#### Scenario: Field order does not refill the bucket
- **WHEN** the hash already stores last as now and tokens as burst, with tokens ordered before last
- **AND** Allow is called burst plus 5 times at one frozen clock with maxDelay 0
- **THEN** exactly burst calls are admitted
- **AND** the rest are denied

#### Scenario: HSET last is persist-max
- **WHEN** the script has a previous hash last and ARGV `t` is earlier than that last
- **THEN** elapsed is clamped so refill is not negative
- **AND** HSET last is the previous last, not raw `t`
