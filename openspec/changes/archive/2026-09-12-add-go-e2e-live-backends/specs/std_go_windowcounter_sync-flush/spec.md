## MODIFIED Requirements

### Requirement: Live tests run on Redis and Dragonfly in CI
Live tests SHALL table-drive Redis and Dragonfly addresses from `WINDOWCOUNTER_LIVE_REDIS` and `WINDOWCOUNTER_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or both addresses are unset. When exactly one address is set they MUST fail. CI `e2e` MUST start both engines, set both addresses, and MUST NOT skip those tests. The unit `test` job MUST pass `-short` and MUST NOT start those engines. The same scenarios SHALL run interpreted (Yaegi) with the compiled test owning start and skip. Traefik and Pester MUST NOT be the behaviour proof.

#### Scenario: Both backends prove exact, buffered share, and sliding boundary
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Redis
- **AND** the same three pass against Dragonfly

#### Scenario: Both backends prove Peek then Take
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** Peek-then-Take (Peek does not increment) passes against Redis
- **AND** the same scenario passes against Dragonfly

#### Scenario: Both backends prove Peek denied then slides
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** Peek stays denied after enough Takes, then becomes allowed as the window slides, against Redis
- **AND** the same scenario passes against Dragonfly

#### Scenario: Both backends prove buffered Peek
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** buffered Peek does not increment and expire-on-first-hit still sets TTL, against Redis
- **AND** the same scenarios pass against Dragonfly
