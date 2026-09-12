## MODIFIED Requirements

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
