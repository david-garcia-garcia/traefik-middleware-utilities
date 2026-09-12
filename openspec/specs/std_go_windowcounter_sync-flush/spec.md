## Purpose

Exact versus buffered Redis sync for the sliding-window limiter: every Take hits Redis when `sync_rate` is zero; a timer flushes local deltas when `sync_rate` is positive. Sleep, Wake, and Close stop the flush goroutine on Traefik reload.

## Requirements

### Requirement: Exact mode increments Redis on every Take
When `sync_rate` is zero, each Take SHALL send Redis `INCR` on the current-window key and SHALL set a TTL of two window lengths when that increment returns 1. Previous-window reads SHALL use `GET` (`redis:miss` counts as zero). Counter updates MUST NOT be a GET-then-SET of the integer.

#### Scenario: Exact mode expires the new window key
- **WHEN** `sync_rate` is 0
- **AND** Take creates a missing current-window key
- **THEN** Redis TTL on that key is two window lengths
- **WHEN** that TTL elapses
- **THEN** a later Take in a new window admits again

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL clear. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms.

#### Scenario: Two clients share the limit
- **WHEN** two limiter instances with two SimpleRedis clients share one opaque key and `sync_rate` greater than zero
- **AND** both flush their local deltas
- **THEN** Redis holds the sum of those deltas
- **AND** a later Take on either instance sees the shared count
- **AND** the last flush MUST NOT overwrite the other client's hits

#### Scenario: Negative sync_rate is rejected
- **WHEN** New is called with a negative sync_rate
- **THEN** construction returns an error
- **AND** no ticker is started

### Requirement: Sleep Wake Close reclaim the flush ticker
The limiter SHALL export `Sleep`, `Wake`, and `Close` suitable for `reclaim.Hooks`. Sleep SHALL flush pending deltas then stop the ticker. Wake SHALL start the ticker when `sync_rate` is greater than zero. Close SHALL run after Sleep and MUST NOT close the injected SimpleRedis client. Exact mode (`sync_rate` zero) SHALL not leak a ticker. The implementation MUST NOT use `time.Tick`.

#### Scenario: Close stops the flush goroutine
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Close is called (after Sleep)
- **THEN** the flush goroutine has exited
- **AND** further Takes MUST NOT start a new ticker
- **AND** the SimpleRedis client remains usable by other callers

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
