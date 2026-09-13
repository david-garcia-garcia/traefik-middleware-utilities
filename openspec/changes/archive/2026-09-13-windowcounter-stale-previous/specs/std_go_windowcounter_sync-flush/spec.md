## ADDED Requirements

### Requirement: Buffered Take refreshes previous when local_delta is zero
When `sync_rate` is greater than zero, Take SHALL GET the previous-window key when that key is already in this instance's buffer as a window it has counted (`expire_at` set) and `local_delta` is 0, then set `redis_known` from that GET (`redis:miss` counts as zero). When `local_delta` is greater than 0, Take SHALL keep the memory count and MUST NOT GET that key (Redis does not hold this instance's unflushed delta). Take MUST NOT GET a previous-window key on every call merely because it was GET-seeded (never counted as current). Take MUST NOT INCR the previous-window key. A failed GET SHALL return on Take the same way a current-window GET failure does. This GET MUST NOT be stored as a retained flush error or deferred to the missed-`sync_rate` probe.

#### Scenario: Two clients then next-window Take denies at three
- **WHEN** two limiter instances with two SimpleRedis clients share one opaque key and `sync_rate` greater than zero
- **AND** each instance Takes once in the same window (limit 2)
- **AND** the first instance Sleeps then the second instance Sleeps
- **AND** Take is called on the first instance at the start of the next window (previous-window weight 1)
- **THEN** that Take returns allowed false
- **AND** the sliding estimate is 3 (current 1 + previous 2)

#### Scenario: Unflushed previous delta is not GET
- **WHEN** `sync_rate` is greater than zero
- **AND** the previous-window key has `local_delta` greater than 0 on this instance
- **THEN** Take MUST NOT GET that previous-window key
- **AND** the estimate uses `redis_known + local_delta` from memory

## MODIFIED Requirements

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL clear. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode returns a retained flush error (or a probe after one missed `sync_rate`) instead of a silent nil.

#### Scenario: Two clients share the limit
- **WHEN** two limiter instances with two SimpleRedis clients share one opaque key and `sync_rate` greater than zero
- **AND** both flush their local deltas
- **THEN** Redis holds the sum of those deltas
- **AND** a later Take on either instance sees the shared count
- **AND** the last flush MUST NOT overwrite the other client's hits

#### Scenario: Shared count survives a window roll
- **WHEN** two limiter instances with two SimpleRedis clients share one opaque key and `sync_rate` greater than zero
- **AND** both flush their local deltas in the same window
- **AND** a later Take on either instance is at the start of the next window so that key is previous
- **THEN** that Take's previous-window term SHALL be the shared Redis count, not this instance's last flush return alone

#### Scenario: Negative sync_rate is rejected
- **WHEN** New is called with a negative sync_rate
- **THEN** construction returns an error
- **AND** no ticker is started

### Requirement: Buffered Peek does not flood Redis during a skip storm
When `sync_rate` is greater than zero, Peek SHALL compute the estimate from `redis_known + local_delta` (plus the sliding previous-window term) under the same lock and store as Take. Peek MUST NOT increment `local_delta`. While Peeking with no Takes, Peek MUST NOT GET Redis on every call merely because `local_delta` is 0. Redis GET for a window key SHALL happen on first sight of that key (including a window roll onto a new current key) and on the existing flush/sync cadence used by buffered Take, not on every Peek. Peek MUST NOT GET the previous-window key on every call to match Take's previous-window GET. The estimate SHALL still age because the previous-window weight uses the call time.

#### Scenario: Skip storm does not GET every Peek
- **WHEN** `sync_rate` is greater than zero
- **AND** Peek is called many times for the same key with zero Takes
- **THEN** Redis GET for that current-window key happens on the first Peek
- **AND** later Peeks in that same window MUST NOT GET Redis again solely because `local_delta` stayed 0

#### Scenario: Peek does not GET previous every call after flush
- **WHEN** `sync_rate` is greater than zero
- **AND** the previous-window key is already in this instance's buffer with `local_delta` 0
- **AND** Peek is called
- **THEN** Peek MUST NOT GET that previous-window key solely because `local_delta` is 0
