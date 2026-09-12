## ADDED Requirements

### Requirement: Failed buffered flush is retained
When `sync_rate` is greater than zero, a failed flush SHALL be stored on the limiter. `Sleep` and `Close` SHALL store that same error instead of discarding it. A later successful flush SHALL clear the stored error. Take and Peek SHALL return the stored error on their existing error result.

#### Scenario: Flush fails then Take sees it
- **WHEN** `sync_rate` is greater than zero
- **AND** a flush of a pending delta fails because Redis is unreachable
- **AND** Take is called
- **THEN** Take returns `redis:unreachable` or `redis:timeout`
- **AND** Peek on that limiter returns the same class of error

### Requirement: Stale buffer probes with one flush
When `sync_rate` is greater than zero and no successful Redis contact has occurred for one `sync_rate` interval, Take and Peek SHALL probe by flushing pending deltas once. They MUST NOT GET Redis on every buffered call merely because a delta is pending. Exact mode (`sync_rate` zero) is unchanged.

#### Scenario: Clock advances one sync_rate after kill
- **WHEN** `sync_rate` is greater than zero and long enough that no tick fires
- **AND** one Take succeeds while Redis is up
- **AND** Redis is then killed
- **AND** the limiter clock advances one `sync_rate`
- **AND** Take is called
- **THEN** Take returns a non-nil Redis error
- **AND** Redis was not GET on every Take between the healthy hit and that probe

#### Scenario: Successful flush then kill still fails closed
- **WHEN** `sync_rate` is greater than zero
- **AND** a Take is followed by a successful flush (`local_delta` cleared)
- **AND** Redis is then killed
- **AND** Take is called
- **THEN** Take returns a non-nil Redis error

### Requirement: Two buffered instances cannot silently multiply the limit
When two limiter instances with two clients share one opaque key and `sync_rate` greater than zero, and Redis is killed mid-window, the combined admitted hits that returned a nil error MUST NOT exceed `limit`. Takes that return a Redis error MAY still carry the local admit decision; those are not silent admits.

#### Scenario: Two instances killed mid-window
- **WHEN** two limiter instances share one key and `sync_rate` greater than zero
- **AND** Redis is killed after each has a pending local delta
- **AND** further Takes are issued on both
- **THEN** the count of Takes that returned a nil error and allowed true is at most `limit`
- **AND** at least one Take after the kill returns a non-nil Redis error

### Requirement: Flush EVAL integer parse keeps the cause
When a buffered flush EVAL reply is not a single integer, the returned error SHALL wrap the conversion failure. Callers that match `redis:issue?` by `Error()` text SHALL still recognize it.

#### Scenario: Non-integer EVAL reply
- **WHEN** a flush EVAL reply cannot be parsed as one integer
- **THEN** the error wraps the conversion failure
- **AND** the error still matches `redis:issue?`

## MODIFIED Requirements

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL clear. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode returns a retained flush error (or a probe after one missed `sync_rate`) instead of a silent nil.

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
