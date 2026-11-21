## Purpose

The status a sliding-window Take or Peek returns: whether the hit is allowed, the sliding estimate, and the limit, remaining hits, period, reset, and retry-after a caller needs to write rate-limit response headers. The library computes these values; it never writes HTTP.

## Requirements

### Requirement: Status reports limit, period, and remaining
`Status.Limit` SHALL be the `limit` argument and `Status.Period` SHALL be the `window` argument. `Status.Remaining` SHALL be `floor(max(limit - Estimated, 0))`, using the `Estimated` that call returns: after this hit for Take and Allow, without a hit for Peek. In both cases it is how many more Takes fit now.

#### Scenario: Takes in a fresh window
- **WHEN** limit is 10, window is 60s, and the key has no prior hits
- **AND** Take is called three times at one clock
- **THEN** the third Take returns `Limit` 10, `Period` 60s, `Estimated` 3, and `Remaining` 7

#### Scenario: Peek with a fractional estimate
- **WHEN** Peek returns `Estimated` equal to `limit - 2.3`
- **THEN** `Remaining` is 2

#### Scenario: Over the limit
- **WHEN** `Estimated` is greater than `limit`
- **THEN** `Remaining` is 0

### Requirement: Reset is the end of the current fixed window
`Status.Reset` SHALL be the time from the clock reading that fixed the window weight to the end of the current epoch-aligned window (`windowStart + window`). It SHALL be in `(0, window]` and SHALL keep the fraction of a second of that clock reading. The previous window's weight is not zero at that instant, so `Reset` MUST NOT be read as the time until the full limit is back.

#### Scenario: Reset keeps the fraction of a second
- **WHEN** window is 60s
- **AND** Take or Peek is called 15.25s after the start of the current window
- **THEN** `Reset` is 44.75s

#### Scenario: Reset at the window start
- **WHEN** Take or Peek is called exactly at the start of a window
- **THEN** `Reset` equals `window`

### Requirement: Retry-after is the first second a Take would be admitted
When `Allowed` is true, `Status.RetryAfter` SHALL be zero, for Peek as well. When `Allowed` is false, `RetryAfter` SHALL be the time from the call's clock reading to the first whole second at which one more Take, with no other traffic, satisfies the admission rule with the same weight function. It SHALL be positive and at most twice `window`. Denied Takes in between still count.

#### Scenario: Denied Take retries in the same window
- **WHEN** limit is 10, window is 10s, the previous window holds 10 hits, and the current window is empty
- **AND** Take is called exactly at the start of the current window
- **THEN** that Take returns `Allowed` false and `Estimated` 11
- **AND** `RetryAfter` is 2s
- **WHEN** the clock advances 2s and Take is called once more
- **THEN** that Take returns `Allowed` true

#### Scenario: Denied Take retries in the next window
- **WHEN** limit is 2, window is 10s, and the previous window is empty
- **AND** Take is called three times 5s after the start of the current window
- **THEN** the third Take returns `Allowed` false
- **AND** `RetryAfter` is 12s (7s into the next window, where the weight of this window's 3 hits is 0.3)

#### Scenario: Denied Peek retries like the next Take
- **WHEN** Peek returns `Allowed` false
- **THEN** `RetryAfter` is the time until a Take with no other traffic would be admitted

#### Scenario: Occupancy at exactly limit
- **WHEN** Peek returns `Estimated` equal to `limit`
- **THEN** Peek returns `Allowed` true, `Remaining` 0, and `RetryAfter` 0
- **AND** the next Take still returns `Allowed` false

### Requirement: Limit below one retries at the window end
When `limit` is less than 1, no Take can be admitted. A denied Take or Peek SHALL then return `RetryAfter` equal to `Reset`, never zero, so a client does not retry in a loop. `limit` is still not validated and `0` MUST NOT mean unlimited.

#### Scenario: Zero limit
- **WHEN** Take is called with limit 0
- **THEN** it returns `Allowed` false
- **AND** `RetryAfter` equals `Reset`

### Requirement: Buffered status is this instance's view
When `sync_rate` is greater than zero, every `Status` field SHALL come from the same counts (`redis_known + local_delta` and the previous-window term) that decide `Allowed` on that call. During a Redis outage the status SHALL describe this instance's per-node cap with a nil error. Take and Peek MUST NOT read Redis only to fill `Status`.

#### Scenario: Buffered outage reports the local cap
- **WHEN** `sync_rate` is greater than zero and Redis is unreachable
- **AND** this instance has admitted `limit - 1` hits in an empty window
- **AND** Take is called
- **THEN** Take returns a nil error, `Allowed` true, and `Remaining` 0

#### Scenario: Status adds no Redis command
- **WHEN** Take or Peek is called in exact or buffered mode
- **THEN** Redis receives only the commands `std_go_windowcounter_sync-flush` prescribes for that mode
- **AND** no GET or EVAL is sent to compute `Remaining`, `Reset`, or `RetryAfter`

### Requirement: Unit and Yaegi prove the status
Compiled tests SHALL prove each `Status` field in exact and buffered mode against the in-process fake Redis, including the same-window and next-window retry-after cases and `limit < 1`. Interpreted tests SHALL read `Status` fields from Take and Peek with stdlib symbols only and `useunsafe` false. Live tests SHALL prove `Remaining`, `Reset`, and `RetryAfter` on a denied Take against Redis and Dragonfly.

#### Scenario: Yaegi reads the status
- **WHEN** interpreted code constructs a limiter on a compiled fake Redis
- **AND** it calls Take until the limit and once more
- **THEN** it reads `Allowed` false, `Remaining` 0, and a positive `RetryAfter` from the returned `Status`
- **AND** Traefik is not started

#### Scenario: Live engines report the status
- **WHEN** CI `e2e-redis` and `e2e-dragonfly` run `go test` without `-short`
- **THEN** a denied Take reports `Remaining` 0, `Reset` in `(0, window]`, and a positive `RetryAfter`, against Redis and against Dragonfly
