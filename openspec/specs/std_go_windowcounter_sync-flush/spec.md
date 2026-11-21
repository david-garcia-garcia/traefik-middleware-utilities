## Purpose

Exact versus buffered Redis sync for the sliding-window limiter: every Take hits Redis when `sync_rate` is zero; a timer flushes local deltas when `sync_rate` is positive. Sleep, Wake, and Close stop the flush timer on Traefik reload.

## Requirements

### Requirement: Exact mode increments Redis on every Take
When `sync_rate` is zero, each Take SHALL increment the current-window key and SHALL set a TTL of two window lengths when that key has no TTL (`PTTL < 0`), including when the increment is not 1. Exact Take SHALL perform that increment and expire in one EVAL with the current-window key declared in `KEYS`. Exact Take MUST NOT refresh TTL when the key already has a TTL. Exact Take MUST NOT delete the key when expire would fail. Previous-window reads SHALL use `GET` (`redis:miss` counts as zero). Counter updates MUST NOT be a GET-then-SET of the integer.

#### Scenario: Exact mode expires the new window key
- **WHEN** `sync_rate` is 0
- **AND** Take creates a missing current-window key
- **THEN** Redis TTL on that key is two window lengths
- **WHEN** that TTL elapses
- **THEN** a later Take in a new window admits again

#### Scenario: Exact mode expires a leftover no-TTL key
- **WHEN** `sync_rate` is 0
- **AND** the current-window key already exists with count at least 1 and no TTL
- **AND** Take is called for that key
- **THEN** Redis TTL on that key is two window lengths
- **AND** the increment is not discarded

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

### Requirement: Flush EVAL does not hold the limiter mutex
When `sync_rate` is greater than zero, the flush EVAL (INCRBY plus EXPIREAT-if-new) MUST NOT run while the limiter mutex that serializes the local window map is held. A Take on an unrelated opaque key MUST be able to finish while that EVAL is in flight.

#### Scenario: Take during an in-flight flush EVAL
- **WHEN** `sync_rate` is greater than zero
- **AND** a flush EVAL for one window key is in flight
- **AND** a Take on a different opaque key is issued on the same limiter
- **THEN** that Take returns without waiting for the EVAL to finish

### Requirement: Buffered Take refreshes previous at most once per sync_rate
When `sync_rate` is greater than zero and the previous-window key has `local_delta` 0, Take SHALL GET that key the first time Take uses it as the previous window on this instance, and after that SHALL GET it again only when at least `sync_rate` has passed since the last such GET (by the limiter clock). The GET result SHALL set `redis_known` (`redis:miss` counts as zero). When `local_delta` is greater than 0, Take SHALL keep the memory count and MUST NOT GET that key (Redis does not hold this instance's unflushed delta). Take MUST NOT GET the previous-window key on every call. Take MUST NOT INCR the previous-window key. The GET MUST NOT hold the limiter mutex. A failed GET SHALL return on Take the same way a current-window GET failure does. This GET MUST NOT be stored as a retained flush error or deferred to the missed-`sync_rate` probe.

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

#### Scenario: Previous refresh is bounded by sync_rate
- **WHEN** `sync_rate` is greater than zero
- **AND** this instance flushed a window that is now previous
- **AND** Take is called 20 times at one clock in the next window
- **THEN** Take GETs previous once, not once per Take
- **WHEN** another instance flushes into that previous key afterwards
- **AND** less than `sync_rate` has passed since that GET
- **THEN** Take MUST NOT GET previous
- **WHEN** `sync_rate` has passed
- **THEN** the next Take GETs previous once and uses the other instance's flush

### Requirement: Failed buffered flush is retained
When `sync_rate` is greater than zero, a failed flush SHALL be stored on the limiter. `Sleep` and `Close` SHALL store that same error instead of discarding it. A later successful flush SHALL clear the stored error. Take and Peek MUST NOT return the stored error. A failed buffered GET SHALL be stored the same way. For one `sync_rate` after the last stored failure (by the limiter clock), buffered Take and Peek MUST NOT GET Redis to refresh the share. After that interval, the next Take or Peek that needs a GET SHALL try Redis again; a successful GET or flush SHALL clear the stored error. A key seeded at zero while Redis was skipped or failing SHALL be GET again on that retry when it has no local or in-flight delta (a key with a delta gets the shared count back from its flush). A Peek-only limiter (no pending delta, so no flush) MUST NOT stay on the outage path after Redis recovers. Exact mode (`sync_rate` zero) is unchanged.

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

#### Scenario: Peek-only limiter recovers after a transient GET failure
- **WHEN** `sync_rate` is greater than zero
- **AND** a buffered GET failure is stored on a limiter that only Peeks
- **AND** Redis is healthy and holds a shared count above `limit` for the key
- **AND** Peek is called within one `sync_rate` of the failure
- **THEN** Peek returns a nil error from this instance's view and does not GET Redis
- **WHEN** the limiter clock advances one `sync_rate` and Peek is called
- **THEN** Peek GETs Redis, returns allowed false at the shared count, and the stored error is cleared

#### Scenario: Take seeded during an outage gets the shared count from flush
- **WHEN** `sync_rate` is greater than zero
- **AND** a Take seeds a key at zero while an outage is stored
- **AND** the next flush succeeds
- **THEN** the stored error is cleared
- **AND** later Takes estimate from the shared Redis count plus local hits

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

### Requirement: Sleep Wake Close reclaim the flush ticker
The limiter SHALL export `Sleep`, `Wake`, and `Close` suitable for `reclaim.Hooks`. Sleep SHALL flush pending deltas then stop the flush timer. Wake SHALL start the timer when `sync_rate` is greater than zero and Sleep or Close is not waiting for an in-flight flush tick. Close SHALL run after Sleep and MUST NOT close the injected SimpleRedis client. Exact mode (`sync_rate` zero) SHALL not leak a timer. The implementation MUST NOT use `time.Tick`. The buffered flush timer MUST be `time.AfterFunc` (or another compiled stdlib waiter). It MUST NOT be an interpreted goroutine that `select`s on a ticker channel and a stop channel. While Sleep or Close is waiting for an in-flight tick, a concurrent Wake MUST NOT start a new timer, and Sleep MUST return. After Sleep returns, a later Wake SHALL start the timer when `sync_rate` is greater than zero. After Close, Wake MUST NOT start a timer.

#### Scenario: Close stops the flush timer
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Close is called (after Sleep)
- **THEN** the flush timer is stopped
- **AND** further Takes MUST NOT start a new timer
- **AND** further Wake MUST NOT start a new timer
- **AND** the SimpleRedis client remains usable by other callers

#### Scenario: Concurrent Wake does not hang Sleep
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep and Wake run at the same time
- **THEN** Sleep returns
- **AND** Wake does not start a flush timer that Sleep waits for

#### Scenario: Wake after Sleep starts the timer
- **WHEN** `sync_rate` is greater than zero and the limiter has been constructed
- **AND** Sleep has returned
- **AND** Close has not been called
- **AND** Wake is called
- **THEN** the flush timer is running

#### Scenario: Interpreted buffered Sleep returns
- **WHEN** `sync_rate` is greater than zero
- **AND** the limiter runs interpreted under Yaegi v0.16.1
- **AND** Sleep is called after buffered Takes
- **THEN** Sleep returns
- **AND** Close after that Sleep returns

### Requirement: Exact-mode Peek reads Redis on every call
When `sync_rate` is zero, each Peek SHALL `GET` the current-window key and the previous-window key (`redis:miss` counts as zero). Peek MUST NOT `INCR`, MUST NOT set TTL, and MUST NOT use a separate consistency mode from Take.

#### Scenario: Exact Peek hits Redis
- **WHEN** `sync_rate` is 0
- **AND** Peek is called
- **THEN** Redis receives GET for the current-window key and the previous-window key
- **WHEN** Peek is called again at the same clock
- **THEN** Redis receives those GETs again

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

### Requirement: Live tests run on Redis and Dragonfly in CI
Live tests SHALL table-drive Redis and Dragonfly addresses from `WINDOWCOUNTER_LIVE_REDIS` and `WINDOWCOUNTER_LIVE_DRAGONFLY`. They SHALL skip when `testing.Short` is set or both addresses are unset. When exactly one address is set they MUST run that engine and MUST NOT fail for the missing engine. CI `e2e-redis` MUST start Redis, set `WINDOWCOUNTER_LIVE_REDIS`, and MUST NOT set `WINDOWCOUNTER_LIVE_DRAGONFLY`. CI `e2e-dragonfly` MUST start Dragonfly, set `WINDOWCOUNTER_LIVE_DRAGONFLY`, and MUST NOT set `WINDOWCOUNTER_LIVE_REDIS`. The unit `test` job MUST pass `-short` and MUST NOT start those engines. The same scenarios SHALL run interpreted (Yaegi) with the compiled test owning start and skip. Traefik and Pester MUST NOT be the behaviour proof.

#### Scenario: Redis job proves exact, buffered share, and sliding boundary
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Redis

#### Scenario: Dragonfly job proves exact, buffered share, and sliding boundary
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** exact N-then-deny, buffered two-client share, and sliding-at-boundary pass against Dragonfly

#### Scenario: Redis job proves Peek then Take
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** Peek-then-Take (Peek does not increment) passes against Redis

#### Scenario: Dragonfly job proves Peek then Take
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** Peek-then-Take (Peek does not increment) passes against Dragonfly

#### Scenario: Both backends prove Peek denied then slides
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** Peek stays denied after enough Takes, then becomes allowed as the window slides, against Redis
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** the same scenario passes against Dragonfly

#### Scenario: Both backends prove buffered Peek
- **WHEN** CI `e2e-redis` runs `go test` without `-short`
- **THEN** buffered Peek does not increment and expire-on-first-hit still sets TTL, against Redis
- **WHEN** CI `e2e-dragonfly` runs `go test` without `-short`
- **THEN** the same scenarios pass against Dragonfly

### Requirement: GET miss matches through wrapping
Previous-window and exact GET misses SHALL be classified with `IsMiss` (or `errors.Is` against the exported miss sentinel). A miss whose `Error()` text is no longer exactly `redis:miss` because it was wrapped SHALL still count as zero. The limiter MUST NOT match miss by `err.Error() ==` the miss token.

#### Scenario: Wrapped miss counts as zero
- **WHEN** GET would return a miss wrapped with `%w`
- **THEN** the limiter treats that counter as zero
- **AND** it does not return a hard error

### Requirement: Two buffered instances each keep their own limit during outage
When two limiter instances with two clients share one opaque key and `sync_rate` is greater than zero, and Redis is killed mid-window, each instance SHALL admit until its own `limit` with a nil error. The sum of those allowed hits MUST NOT be treated as a global cap. Take and Peek MUST NOT return a Redis error to enforce a combined limit.

#### Scenario: Two instances killed mid-window
- **WHEN** two limiter instances share one key and `sync_rate` is greater than zero
- **AND** Redis is killed after each has a pending local delta
- **AND** further Takes are issued on both
- **THEN** each instance's Takes that returned allowed true and a nil error are at most that instance's `limit`
- **AND** Take after the kill returns a nil error

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
