## Purpose

Exact versus buffered Redis sync for the leaky-bucket limiter: every pour is a leak-then-add EVAL when `sync_rate` is zero; a timer flushes local pours when `sync_rate` is positive. Sleep, Wake, and Close stop the flush goroutine on Traefik reload.

## Requirements

### Requirement: Exact mode leaks then adds in one EVAL
When `sync_rate` is zero, each Add or Take SHALL send Redis `EVAL` that leaks stored water once using the caller's now, then adds the pour if it would not exceed `capacity`, then stores HASH fields `water` and `last` and `EXPIRE`s the key. The touched key MUST be listed in `KEYS`. The script MUST NOT use `table.maxn`. Go MUST NOT `SET` or `HSET` that hash outside the script. Level in exact mode SHALL EVAL with a zero pour (leak only).

#### Scenario: Exact mode pour to cap then leak then Take
- **WHEN** `sync_rate` is 0
- **AND** Take is called `capacity` times on an empty key
- **THEN** each of those Takes returns allowed true
- **WHEN** Take is called once more before leak time
- **THEN** that Take returns allowed false
- **WHEN** `capacity / leak` seconds pass
- **AND** Take is called again
- **THEN** that Take returns allowed true

### Requirement: Buffered mode shares water without last-write-wins
When `sync_rate` is greater than zero, Add SHALL admit from leaked(`redis_water`, `last_sync`, now) plus `local_pours` and SHALL NOT write Redis on every Add. A timer SHALL flush pending `local_pours` with one EVAL that leaks once then adds the delta and stores `{water, last}`. After a successful flush, local state SHALL become the EVAL water and last, and `local_pours` SHALL clear. `sync_rate` less than zero SHALL fail construction. `sync_rate` greater than zero and less than 20 ms SHALL floor to 20 ms. Go MUST NOT `SET` a replica's `{water, last}` blob.

#### Scenario: Two clients both pours count
- **WHEN** two limiter instances with two SimpleRedis clients share one opaque key and `sync_rate` greater than zero
- **AND** each instance Takes once
- **AND** both flush
- **THEN** Redis water is the sum of those pours (both count)
- **AND** the last flush MUST NOT overwrite the other client's pours

#### Scenario: Over-allow is bounded by the sync interval
- **WHEN** two limiter instances share one key at `sync_rate` greater than zero
- **AND** both admit from a stale leaked water before either flushes
- **THEN** extra admits beyond `capacity` MUST NOT grow without bound as wall time passes without a flush
- **AND** after both flush, further Takes see the shared water

#### Scenario: Negative sync_rate is rejected
- **WHEN** NewRedis is called with a negative sync_rate
- **THEN** construction returns an error
- **AND** no ticker is started

### Requirement: Memory and Redis agree when exact
For the same leak, capacity, ttl, and pour sequence with `sync_rate` 0, the in-memory store and the Redis store SHALL return the same allow/deny and the same water class (empty, partial, full) on each step.

#### Scenario: Same sequence on both stores
- **WHEN** NewMemory and NewRedis (`sync_rate` 0) share leak, capacity, and ttl
- **AND** the same Add/Take/Level sequence runs on both with the same clock
- **THEN** each step's allowed matches
- **AND** each step's level is either both empty, both between 0 and capacity exclusive, or both full

### Requirement: Sleep Wake Close reclaim the flush ticker
The Redis limiter SHALL export `Sleep`, `Wake`, and `Close` suitable for `reclaim.Hooks`. Sleep SHALL flush pending pours then stop the ticker. Wake SHALL start the ticker when `sync_rate` is greater than zero. Close SHALL run after Sleep and MUST NOT close the injected SimpleRedis client. Exact mode (`sync_rate` zero) SHALL not leak a ticker. The implementation MUST NOT use `time.Tick`. The memory store MUST NOT export those hooks.

#### Scenario: Close stops the flush goroutine
- **WHEN** `sync_rate` is greater than zero and the Redis limiter has been constructed
- **AND** Close is called (after Sleep)
- **THEN** the flush goroutine has exited
- **AND** further Adds MUST NOT start a new ticker
- **AND** the SimpleRedis client remains usable by other callers

### Requirement: Redis errors do not become deny
When Redis is unreachable or times out, Add, Take, and Level SHALL return that error (`redis:unreachable` or `redis:timeout`). The library MUST NOT fail-open, fail-close, or denyOnError inside those calls.

#### Scenario: Unreachable Redis
- **WHEN** Take cannot complete the script because the server is unreachable
- **THEN** Take returns `redis:unreachable`
- **AND** does not return allowed false as a substitute

### Requirement: Live Redis and Dragonfly prove both engines
Compiled live tests SHALL call Add, Take, and Level against Redis and Dragonfly using table-driven addresses from `LEAKYBUCKET_LIVE_REDIS` and `LEAKYBUCKET_LIVE_DRAGONFLY`. They SHALL skip when those env vars are unset or when tests run under `-short`. CI SHALL set both env vars on the existing engine services and MUST NOT skip. Yaegi live SHALL run the same Take scenarios interpreted (stdlib only, `useunsafe` false); the compiled test owns start/skip.

#### Scenario: Both engines in CI
- **WHEN** CI runs the package tests without `-short`
- **AND** both live env vars point at Redis `:6379` and Dragonfly `:6380`
- **THEN** the live cases run on both addresses
- **AND** they MUST NOT skip
