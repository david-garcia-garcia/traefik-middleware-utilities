## MODIFIED Requirements

### Requirement: Eval runs the copied Traefik script
The Redis store SHALL admit via `simpleredis.Eval` of the Traefik `AllowTokenBucketRaw` script with Traefik Labs MIT notice kept, plus this product's empty-hash fill: when `HGETALL` is not four fields (`#rl_source ~= 4`), the script SHALL set tokens to burst and last to now (elapsed 0) before the existing refill and consume. The hash key SHALL be listed in `KEYS`. The script SHALL use `#rl_source == 4` (not `table.maxn`). Go MUST NOT GET or SET that hash. v1 MUST NOT use EVALSHA. ARGV SHALL pass `rate/1e6`, burst, ttl seconds, Unix microseconds, and maxDelay microseconds.

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
