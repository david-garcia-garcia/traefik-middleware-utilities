## ADDED Requirements

### Requirement: Flush EVAL does not hold the limiter mutex
When `sync_rate` is greater than zero, the flush EVAL (INCRBY plus EXPIREAT-if-new) MUST NOT run while the limiter mutex that serializes the local window map is held. A Take on an unrelated opaque key MUST be able to finish while that EVAL is in flight.

#### Scenario: Take during an in-flight flush EVAL
- **WHEN** `sync_rate` is greater than zero
- **AND** a flush EVAL for one window key is in flight
- **AND** a Take on a different opaque key is issued on the same limiter
- **THEN** that Take returns without waiting for the EVAL to finish

## MODIFIED Requirements

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL decrease by the amount sent on that EVAL. A hit counted into `local_delta` while that EVAL was in flight MUST remain in `local_delta`. The same snapshot MUST NOT be EVAL'd twice while it is in flight. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode returns a retained flush error (or a probe after one missed `sync_rate`) instead of a silent nil.

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

#### Scenario: Take during flush keeps the new local hit
- **WHEN** `sync_rate` is greater than zero
- **AND** a window has a pending `local_delta` of N
- **AND** flush sends N on EVAL
- **AND** a Take increments `local_delta` while that EVAL is in flight
- **AND** the EVAL succeeds with integer return M
- **THEN** `redis_known` is M
- **AND** `local_delta` is the amount added during the EVAL, not zero
