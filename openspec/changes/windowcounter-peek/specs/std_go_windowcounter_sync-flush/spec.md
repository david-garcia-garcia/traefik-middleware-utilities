## ADDED Requirements

### Requirement: Exact-mode Peek reads Redis on every call
When `sync_rate` is zero, each Peek SHALL `GET` the current-window key and the previous-window key (`redis:miss` counts as zero). Peek MUST NOT `INCR`, MUST NOT set TTL, and MUST NOT use a separate consistency mode from Take.

#### Scenario: Exact Peek hits Redis
- **WHEN** `sync_rate` is 0
- **AND** Peek is called
- **THEN** Redis receives GET for the current-window key and the previous-window key
- **WHEN** Peek is called again at the same clock
- **THEN** Redis receives those GETs again

### Requirement: Buffered Peek does not flood Redis during a skip storm
When `sync_rate` is greater than zero, Peek SHALL compute the estimate from `redis_known + local_delta` (plus the sliding previous-window term) under the same lock and store as Take. Peek MUST NOT increment `local_delta`. While Peeking with no Takes, Peek MUST NOT GET Redis on every call merely because `local_delta` is 0. Redis GET for a window key SHALL happen on first sight of that key (including a window roll onto a new key) and on the existing flush/sync cadence used by buffered Take, not on every Peek. The estimate SHALL still age because the previous-window weight uses the call time.

#### Scenario: Skip storm does not GET every Peek
- **WHEN** `sync_rate` is greater than zero
- **AND** Peek is called many times for the same key with zero Takes
- **THEN** Redis GET for that current-window key happens on the first Peek
- **AND** later Peeks in that same window MUST NOT GET Redis again solely because `local_delta` stayed 0

## MODIFIED Requirements

### Requirement: Live tests run on Redis and Dragonfly in CI
Live tests SHALL table-drive Redis and Dragonfly addresses from `WINDOWCOUNTER_LIVE_REDIS` and `WINDOWCOUNTER_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or an address is missing. CI MUST start both engines, set both addresses, and MUST NOT skip those tests. The same scenarios SHALL run interpreted (Yaegi) with the compiled test owning start and skip. Traefik and Pester MUST NOT be the behaviour proof.

#### Scenario: Both backends prove exact, buffered share, and sliding boundary
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Redis
- **AND** the same three pass against Dragonfly

#### Scenario: Both backends prove Peek then Take
- **WHEN** CI runs `go test` without `-short` with both live addrs set
- **THEN** Peek-then-Take (Peek does not increment) passes against Redis
- **AND** the same scenario passes against Dragonfly
