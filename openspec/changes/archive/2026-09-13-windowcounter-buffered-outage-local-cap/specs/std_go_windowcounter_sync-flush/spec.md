## ADDED Requirements

### Requirement: Two buffered instances each keep their own limit during outage
When two limiter instances with two clients share one opaque key and `sync_rate` is greater than zero, and Redis is killed mid-window, each instance SHALL admit until its own `limit` with a nil error. The sum of those allowed hits MUST NOT be treated as a global cap. Take and Peek MUST NOT return a Redis error to enforce a combined limit.

#### Scenario: Two instances killed mid-window
- **WHEN** two limiter instances share one key and `sync_rate` is greater than zero
- **AND** Redis is killed after each has a pending local delta
- **AND** further Takes are issued on both
- **THEN** each instance's Takes that returned allowed true and a nil error are at most that instance's `limit`
- **AND** Take after the kill returns a nil error

## MODIFIED Requirements

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL clear. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode during a Redis outage keeps this instance's `limit` with a nil error (per-node cap), not a retained flush error returned from Take or Peek.

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

### Requirement: Failed buffered flush is retained
When `sync_rate` is greater than zero, a failed flush SHALL be stored on the limiter. `Sleep` and `Close` SHALL store that same error instead of discarding it. A later successful flush SHALL clear the stored error. Take and Peek MUST NOT return the stored error. While that error is stored, buffered Take and Peek MUST NOT GET Redis to refresh the share and MUST NOT probe with EVAL or GET after one missed `sync_rate`. Exact mode (`sync_rate` zero) is unchanged.

#### Scenario: Flush fails then Take stays local
- **WHEN** `sync_rate` is greater than zero
- **AND** a flush of a pending delta fails because Redis is unreachable
- **AND** Take is called
- **THEN** Take returns a nil error
- **AND** Peek on that limiter also returns a nil error
- **AND** Redis was not GET on that Take solely to surface the outage

#### Scenario: Clock advances one sync_rate after kill
- **WHEN** `sync_rate` is greater than zero and long enough that no tick fires
- **AND** one Take succeeds while Redis is up
- **AND** Redis is then killed
- **AND** the limiter clock advances one `sync_rate`
- **AND** Take is called
- **THEN** Take returns a nil error
- **AND** Redis was not GET on every Take between the healthy hit and that call
- **AND** Redis was not INCR on that Take

## REMOVED Requirements

### Requirement: Stale buffer probes with one flush
**Reason**: Dest probed Take/Peek after one missed `sync_rate` so a Redis error would fail closed. The accepted contract is the per-node cap with a nil error; probing on Take/Peek is forbidden.
**Migration**: Use “Failed buffered flush is retained” (store the error so later Take/Peek skip Redis) and sliding-take “Buffered outage is a per-node cap with a nil error”.

### Requirement: Two buffered instances cannot silently multiply the limit
**Reason**: Dest required combined nil-error admits to stay at `limit` and at least one Redis error after kill. The accepted contract is each instance's own `limit` with nil errors.
**Migration**: Use “Two buffered instances each keep their own limit during outage”.
