## MODIFIED Requirements

### Requirement: Live tests run on Redis and Dragonfly in CI
Live tests SHALL table-drive Redis and Dragonfly addresses from `WINDOWCOUNTER_LIVE_REDIS` and `WINDOWCOUNTER_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or both addresses are unset. When exactly one address is set they MUST run that engine and MUST NOT fail for the missing engine. CI `e2e-redis` MUST start Redis, set `WINDOWCOUNTER_LIVE_REDIS`, and MUST NOT set `WINDOWCOUNTER_LIVE_DRAGONFLY`. CI `e2e-dragonfly` MUST start Dragonfly, set `WINDOWCOUNTER_LIVE_DRAGONFLY`, and MUST NOT set `WINDOWCOUNTER_LIVE_REDIS`. The unit `test` job MUST pass `-short` and MUST NOT start those engines. The same scenarios SHALL run interpreted (Yaegi) with the compiled test owning start and skip. Traefik and Pester MUST NOT be the behaviour proof.

#### Scenario: Redis job proves exact, buffered share, and sliding boundary
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Redis

#### Scenario: Dragonfly job proves exact, buffered share, and sliding boundary
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Dragonfly

#### Scenario: Redis job proves Peek then Take
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** Peek-then-Take (Peek does not increment) passes against Redis

#### Scenario: Dragonfly job proves Peek then Take
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** Peek-then-Take (Peek does not increment) passes against Dragonfly
