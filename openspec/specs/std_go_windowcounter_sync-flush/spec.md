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
When `sync_rate` is greater than zero, Take SHALL admit from `redis_known + local_delta` (plus the sliding previous-window term) and SHALL NOT `INCR` on every Take. A timer SHALL flush pending deltas with one EVAL of INCRBY plus EXPIREAT when the key did not exist, with the touched key declared in `KEYS`. After a successful flush, `redis_known` SHALL become the EVAL return and `local_delta` SHALL decrease by the amount sent on that EVAL. A hit counted into `local_delta` while that EVAL was in flight MUST remain in `local_delta`. The same snapshot MUST NOT be EVAL'd twice while it is in flight. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Construction and the README SHALL state that exact mode returns Redis errors on every Take, and that buffered mode returns a retained flush error (or a probe after one missed `sync_rate`) instead of a silent nil.

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
- **AND** the EVAL succeeds with integer return M
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
