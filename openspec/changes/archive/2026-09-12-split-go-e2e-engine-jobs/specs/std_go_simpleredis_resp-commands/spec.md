## MODIFIED Requirements

### Requirement: Live Redis and Dragonfly prove Lua MSetEX TTL landed
Compiled tests SHALL run `MSetEX` against each of Redis 7 and Dragonfly whose live address is set (`SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`). Those tests MUST skip under `-short` or when both addresses are unset. When exactly one address is set they MUST run that engine and MUST NOT fail for the missing engine. After `MSetEX`, `Get` SHALL return the written bytes and `Eval` of `TTL` on a declared KEYS key SHALL return a positive integer. `MSetEXAt` with a past timestamp SHALL then `Get` as a miss. CI `e2e-redis` MUST set `SIMPLEREDIS_LIVE_REDIS` to Redis 7 `:6379` and MUST NOT set `SIMPLEREDIS_LIVE_DRAGONFLY`. CI `e2e-dragonfly` MUST set `SIMPLEREDIS_LIVE_DRAGONFLY` to Dragonfly `:6380` and MUST NOT set `SIMPLEREDIS_LIVE_REDIS`. The unit `test` job MUST NOT set those vars. Pester on `/redis` and `/dragonfly` is not a substitute for this compiled live file.

#### Scenario: Live Redis TTL landed
- **WHEN** `SIMPLEREDIS_LIVE_REDIS` is set and tests are not `-short`
- **AND** MSetEX writes a key with a positive duration
- **THEN** Get returns the written bytes
- **AND** Eval of TTL for that key in KEYS returns a positive integer

#### Scenario: Live Dragonfly TTL landed
- **WHEN** `SIMPLEREDIS_LIVE_DRAGONFLY` is set and tests are not `-short`
- **AND** MSetEX writes a key with a positive duration
- **THEN** Get returns the written bytes
- **AND** Eval of TTL for that key in KEYS returns a positive integer

#### Scenario: Live past EXAT is a miss
- **WHEN** a live address is set
- **AND** MSetEXAt is called with a Unix timestamp in the past
- **THEN** a later Get of that key is `redis:miss`
