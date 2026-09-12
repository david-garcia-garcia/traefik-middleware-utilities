## Purpose

Compiled and Yaegi SimpleRedis tests against live Redis 7 and Dragonfly for every engine-success verb; fake TCP remains the proof for malformed replies. SELECT 99 and WRONGPASS are live Go E2E.

### Requirement: Live Redis and Dragonfly prove engine-success verbs
Compiled tests SHALL run Get hit and miss, Set with EX then Get, Del, MGet hits misses and empty, Incr then IncrBy, Expire then TTL via Eval, Eval integer, a second Eval that hits EVALSHA, Eval after SCRIPT FLUSH (NOSCRIPT then EVAL), existing MSetEX TTL and past EXAT, pool-wait `redis:unreachable`, CLIENT KILL then Get, and SELECT 99 (`ERR DB index is out of range`, idle empty), against both Redis 7 and Dragonfly when both `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY` are set. Wrong-password tests SHALL run against both `SIMPLEREDIS_LIVE_REDIS_AUTH` and `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` and MUST map WRONGPASS to `redis:noauth` with idle empty. Those tests MUST skip under `-short` or when both addresses in that pair are unset. When exactly one address in a pair is set they MUST fail. Pester on `/redis` and `/dragonfly` is not a substitute. Malformed RESP, truncated replies, LOADING retry, and AUTH-class prefixes dest engines do not emit MUST remain fake-TCP only.

#### Scenario: Live Redis verbs
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** Get, Set, Del, MGet, Incr, IncrBy, Expire, Eval, EVALSHA, NOSCRIPT fallback, MSetEX, pool wait, and CLIENT KILL recovery pass against Redis

#### Scenario: Live Dragonfly verbs
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** the same cases pass against Dragonfly

#### Scenario: Live SELECT 99
- **WHEN** both SimpleRedis dest live addresses are set and tests are not `-short`
- **THEN** SELECT database `99` returns `ERR DB index is out of range` and the socket is not pooled on Redis and Dragonfly

#### Scenario: Live WRONGPASS
- **WHEN** both SimpleRedis AUTH live addresses are set and tests are not `-short`
- **THEN** a wrong password maps to `redis:noauth` and the socket is not pooled on Redis and Dragonfly

#### Scenario: Fake-TCP keeps peer-abuse
- **WHEN** `go test -short ./simpleredis` runs
- **THEN** malformed and truncated RESP tests still run
- **AND** live tests skip

### Requirement: Yaegi live SimpleRedis on both engines
Tests that import Yaegi v0.16.1 SHALL prove interpreted `New` plus Get, Set, Del, Incr, Eval, and MSetEX against live Redis and Dragonfly with the compiled test owning start and skip. Those tests MUST use GOPATH with stdlib only and `useunsafe` false. They MUST NOT start Traefik. Skip and fail rules SHALL match the compiled live file.

#### Scenario: Interpreted live Redis
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** the Yaegi live probe passes against Redis

#### Scenario: Interpreted live Dragonfly
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** the Yaegi live probe passes against Dragonfly
