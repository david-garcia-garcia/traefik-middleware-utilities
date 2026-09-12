## MODIFIED Requirements

### Requirement: Live Redis and Dragonfly
Live tests SHALL call Allow against Redis and Dragonfly using `TOKENBUCKET_LIVE_REDIS` and `TOKENBUCKET_LIVE_DRAGONFLY`. Tests SHALL skip when both addrs are unset or under `-short`. When exactly one address is set they MUST fail. CI `e2e` SHALL start both engines, set both variables, and MUST NOT skip. The unit `test` job MUST pass `-short` and MUST NOT start those engines. Yaegi live SHALL run the same scenarios; the compiled test owns start/skip; the interpreted probe calls Allow. Live scenarios SHALL include burst after idle, two-instance share, memory/Redis agreement, and refund when wait exceeds maxDelay.

#### Scenario: Both engines table-driven
- **WHEN** CI runs `go test` without `-short`
- **AND** both live env vars are set
- **THEN** Allow scenarios run against Redis and against Dragonfly
- **AND** the live tests MUST NOT skip

#### Scenario: Both engines prove refund past maxDelay
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** Allow after a wait greater than maxDelay does not admit a stacked consume against Redis
- **AND** the same scenario passes against Dragonfly
