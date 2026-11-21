## MODIFIED Requirements

### Requirement: Buffered mode shares one limit without last-write-wins
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with EVALs that each, for every window key they carry, INCRBY that key and EXPIREAT it when the key did not exist, with every touched key declared in `KEYS`. After a key's flush is applied, `redis_known` SHALL become that key's INCRBY result and `local_delta` SHALL decrease by the amount sent for that key. A hit counted into `local_delta` while that EVAL was in flight MUST remain in `local_delta`. The same snapshot MUST NOT be EVAL'd twice while it is in flight. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode during a Redis outage keeps this instance's `limit` with a nil error (per-node cap), not a retained flush error returned from Take or Peek.

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

#### Scenario: Take during flush keeps the new local hit
- **WHEN** `sync_rate` is greater than zero
- **AND** a window has a pending `local_delta` of N
- **AND** flush sends N on EVAL
- **AND** a Take increments `local_delta` while that EVAL is in flight
- **AND** the EVAL reports that key applied with INCRBY result M
- **THEN** `redis_known` is M
- **AND** `local_delta` is the amount added during the EVAL, not zero

### Requirement: Flush EVAL integer parse keeps the cause
When a key's slot in a buffered flush EVAL reply is neither an integer nor that key's failure report, or the reply does not hold one slot per key the EVAL carried, the returned error SHALL wrap the conversion failure when there is one. Callers that match `redis:issue?` by `Error()` text SHALL still recognize it. A key whose slot cannot be read SHALL keep its `local_delta` for the next flush.

#### Scenario: Non-integer EVAL reply
- **WHEN** a key's slot in a flush EVAL reply cannot be parsed as one integer
- **THEN** the error wraps the conversion failure
- **AND** the error still matches `redis:issue?`
- **AND** that key keeps its `local_delta`

#### Scenario: Reply without one slot per key
- **WHEN** a flush EVAL reply holds a different number of slots than the keys that EVAL carried
- **THEN** the error matches `redis:issue?`
- **AND** every key that EVAL carried keeps its `local_delta`

## ADDED Requirements

### Requirement: Buffered flush batches keys with a per-key outcome
When `sync_rate` is greater than zero, one flush EVAL SHALL carry at most 1024 window keys and SHALL report one outcome per key. A key reported applied SHALL leave the pending set and MUST NOT be sent again for that delta. A key reported failed, and every key of an EVAL that failed as a whole, SHALL keep its `local_delta` for the next flush. A failure on one key MUST NOT change another key's outcome. The first failure of a flush SHALL be stored.

#### Scenario: Many dirty keys flush in fewer EVALs than keys
- **WHEN** `sync_rate` is greater than zero
- **AND** 1000 window keys have a pending `local_delta`
- **AND** a flush runs against a healthy engine
- **THEN** Redis receives fewer flush EVALs than keys
- **AND** no flush EVAL declares more than 1024 keys in `KEYS`
- **AND** each key's Redis count increases by its delta exactly once

#### Scenario: One failing key does not fail the others
- **WHEN** one flush EVAL carries keys A and B
- **AND** the engine applies A's increment and fails B's
- **THEN** A's `redis_known` is A's INCRBY result and A's `local_delta` drops by the amount sent
- **AND** B keeps its `local_delta`
- **AND** the next flush sends B's delta and does not send A's delta again
- **AND** the stored error is B's failure
- **AND** a Take after that flush returns a nil error

#### Scenario: Whole EVAL failure keeps every delta
- **WHEN** a flush EVAL fails as a whole because Redis is unreachable or times out
- **THEN** every key carried by that EVAL keeps its `local_delta`
- **AND** the error is stored
- **AND** a later successful flush sends each of those deltas once

#### Scenario: Several keys in one EVAL on both engines
- **WHEN** CI `e2e-redis` and `e2e-dragonfly` run `go test` without `-short`
- **THEN** a buffered flush that carries several keys in one EVAL applies every key's delta, against Redis and against Dragonfly
