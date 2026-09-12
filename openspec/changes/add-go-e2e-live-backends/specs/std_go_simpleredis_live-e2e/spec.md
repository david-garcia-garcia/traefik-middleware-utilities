## Purpose

Compiled and Yaegi SimpleRedis tests against live Redis 7 and Dragonfly for every engine-success verb; fake TCP remains the proof for malformed replies and AUTH.

## ADDED Requirements

### Requirement: Live Redis and Dragonfly prove engine-success verbs
Compiled tests SHALL run Get hit and miss, Set with EX then Get, Del, MGet hits misses and empty, Incr then IncrBy, Expire then TTL via Eval, Eval integer, a second Eval that hits EVALSHA, Eval after SCRIPT FLUSH (NOSCRIPT then EVAL), existing MSetEX TTL and past EXAT, pool-wait `redis:unreachable`, and CLIENT KILL then Get, against both Redis 7 and Dragonfly when both `SIMPLEREDIS_LIVE_REDIS` and `SIMPLEREDIS_LIVE_DRAGONFLY` are set. Those tests MUST skip under `-short` or when both addresses are unset. When exactly one address is set they MUST fail. Pester on `/redis` and `/dragonfly` is not a substitute. Malformed RESP, truncated replies, LOADING retry, AUTH, and SELECT MUST remain fake-TCP only.

#### Scenario: Live Redis verbs
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** Get, Set, Del, MGet, Incr, IncrBy, Expire, Eval, EVALSHA, NOSCRIPT fallback, MSetEX, pool wait, and CLIENT KILL recovery pass against Redis

#### Scenario: Live Dragonfly verbs
- **WHEN** both SimpleRedis live addresses are set and tests are not `-short`
- **THEN** the same cases pass against Dragonfly

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
