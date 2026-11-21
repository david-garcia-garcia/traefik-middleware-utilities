## Purpose

The status a token-bucket Allow returns: whether the consume is allowed plus the limit, remaining tokens, period, reset, delay, and retry-after a caller needs to write rate-limit response headers. The library computes these values; it never writes HTTP.

## Requirements

### Requirement: Status reports the bucket after this Allow
`Status.Limit` SHALL be `burst`. `Status.Remaining` SHALL be `floor(max(tokens, 0))`, where `tokens` is what the store keeps after this Allow (after the refund when denied). `Status.Period` SHALL be `burst / rate`, the time to refill an empty bucket. `Status.Reset` SHALL be `max(burst - tokens, 0) / rate`, the time until the bucket is full again. `Remaining` SHALL never exceed `Limit`.

#### Scenario: First Allow on a new key
- **WHEN** rate is 10 per second and burst is 5
- **AND** Allow is called once on a new key
- **THEN** `Limit` is 5
- **AND** `Remaining` is 4
- **AND** `Period` is 500ms
- **AND** `Reset` is 100ms

#### Scenario: Fractional tokens round down
- **WHEN** the tokens kept after an Allow are 2.7
- **THEN** `Remaining` is 2

#### Scenario: Debt reports zero remaining
- **WHEN** an Allow is admitted with a positive `Delay` so the tokens kept are negative
- **THEN** `Remaining` is 0
- **AND** `Reset` is `(burst - tokens) / rate`, more than `Period`

#### Scenario: Denied Allow reports the refunded bucket
- **WHEN** an Allow is denied and its consume is refunded
- **THEN** `Remaining` and `Reset` are computed from the refunded tokens
- **AND** a following Allow at the same instant reports the same `Remaining`

### Requirement: Status splits the wait into delay and retry-after
When `Allowed` is true, `Status.Delay` SHALL be the wait (zero when tokens did not go negative) and `Status.RetryAfter` SHALL be zero. When `Allowed` is false, `Status.RetryAfter` SHALL be the wait and `Status.Delay` SHALL be zero. `Delay + RetryAfter` SHALL equal the wait the store computed. The caller sleeps `Delay` to keep Traefik's delayed admit; the library MUST NOT sleep.

#### Scenario: Admitted with a delay
- **WHEN** `maxDelay` is 2s, burst is 1, and rate is 1 per second
- **AND** Allow is called twice on a new key at the same instant
- **THEN** the second Allow returns `Allowed` true, `Delay` 1s, and `RetryAfter` 0

#### Scenario: Denied with retry-after
- **WHEN** `maxDelay` is 0, burst is 5, and rate is 1 per second
- **AND** Allow is called six times on a new key at the same instant
- **THEN** the sixth Allow returns `Allowed` false, `RetryAfter` 1s, and `Delay` 0

#### Scenario: Admitted without a delay
- **WHEN** Allow is admitted and tokens kept are at least 0
- **THEN** `Delay` and `RetryAfter` are both 0

### Requirement: Status durations do not overflow
`Period`, `Reset`, `Delay`, and `RetryAfter` SHALL be the maximum `time.Duration` when the computed value is larger than a `time.Duration` can hold, and SHALL be zero when the computed value is not positive. A duration field MUST NOT be negative. Saturation MUST NOT change `Allowed`.

#### Scenario: Huge burst over a tiny rate
- **WHEN** burst is `1e12` and rate is `0.001` per second
- **AND** Allow is called on a new key
- **THEN** `Period` is the maximum `time.Duration`
- **AND** no duration field is negative

#### Scenario: Huge wait on deny
- **WHEN** the wait in microseconds is larger than the maximum `time.Duration` can hold
- **AND** Allow is denied
- **THEN** `RetryAfter` is the maximum `time.Duration`
- **AND** `Allowed` is false

### Requirement: Memory and Redis status agree
For the same rate, burst, maxDelay, ttl, and Allow sequence (same test clock), the in-memory and Redis stores SHALL return equal `Allowed`, `Limit`, and `Period`, and the same class for `Delay` and `RetryAfter` (zero, positive at most maxDelay, or greater than maxDelay). `Remaining` and `Reset` SHALL be equal when the tokens kept stay whole numbers. Otherwise they MAY differ only by the 14-significant-digit tokens text the Redis script returns.

#### Scenario: Same sequence, same status
- **WHEN** a memory limiter and a Redis limiter share rate, burst, maxDelay, and ttl
- **AND** the same Allow sequence with whole-number tokens runs on both with aligned clocks
- **THEN** each step's `Allowed`, `Limit`, `Remaining`, `Period`, and `Reset` match
- **AND** each step's `Delay` and `RetryAfter` fall in the same class on both stores

### Requirement: Unit and Yaegi prove the status
Compiled tests SHALL prove each `Status` field on the in-memory store and on the Redis store against a fake TCP Redis, including a malformed tokens field. Interpreted tests SHALL read `Status` fields from Allow with stdlib symbols only and `useunsafe` false. Live tests SHALL prove memory/Redis status agreement against Redis and Dragonfly.

#### Scenario: Yaegi reads the status
- **WHEN** the Yaegi GOPATH interp loads non-test `tokenbucket` and `simpleredis` sources
- **AND** the probe calls Allow on a memory limiter and on a Redis limiter against the compiled fake
- **THEN** the probe reads `Allowed`, `Remaining`, and `RetryAfter` from the returned `Status`
- **AND** the interp MUST NOT enable unsafe

#### Scenario: Live engines agree on the status
- **WHEN** CI `e2e-redis` and `e2e-dragonfly` run `go test` without `-short`
- **THEN** the memory/Redis status agreement scenario passes against Redis and against Dragonfly
