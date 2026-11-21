## MODIFIED Requirements

### Requirement: Take admits until the sliding estimate exceeds the limit
`Take(ctx, key, limit, window)` SHALL increment the current-window counter, then return a `Status` and an error. `Status.Allowed` SHALL be true when the sliding estimate is less than or equal to `limit`, and false otherwise. `Status.Estimated` SHALL be that sliding estimate. The other `Status` fields follow `std_go_windowcounter_status`. When the error is non-nil, the `Status` SHALL be the zero value. `Allow` SHALL be the same operation. The first argument SHALL be a `context.Context`. A caller with no deadline SHALL pass `context.Background()`. The estimate SHALL be `current + previous × (1 − elapsed/window)` using the caller's opaque `key`. Denied hits SHALL still occupy the window. Prefixing of `key` is the caller's job.

#### Scenario: N Takes then deny
- **WHEN** the limit is N and the window has no prior hits
- **AND** Take is called N times for the same key
- **THEN** each of those Takes returns `Allowed` true
- **WHEN** Take is called once more for that key in the same window
- **THEN** that Take returns `Allowed` false

#### Scenario: Caller owns the key
- **WHEN** Take is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

#### Scenario: Error returns the zero status
- **WHEN** Take, Peek, or Allow returns a non-nil error (cancelled context, invalid window, or an exact-mode Redis error)
- **THEN** the returned `Status` is the zero value
- **AND** its `Allowed` is false

### Requirement: Sliding estimate uses current and previous windows
The current window start SHALL be `floor(unixSeconds / windowSeconds) × windowSeconds`. Redis keys SHALL be `{opaqueKey}:{windowStart}` and `{opaqueKey}:{previousWindowStart}`. A missing previous window SHALL count as zero. Window length SHALL be a whole number of seconds. Sub-second windows MUST NOT be supported. Take and Peek MUST return an error when `window` is shorter than one second or is not an integer number of seconds (for example 1500ms). They MUST NOT accept that window and silently use truncated whole-second buckets. Weight and TTL SHALL use that whole-second length only.

#### Scenario: Dump at the window boundary does not double the limit
- **WHEN** a key has used its full limit near the end of a window
- **AND** Take is called at the start of the next window
- **THEN** the estimate still includes the previous window's hits
- **AND** the new window MUST NOT admit a second full limit the way a fixed window would

#### Scenario: Buffered two-client dump at the window boundary
- **WHEN** two limiter instances with `sync_rate` greater than zero each record one hit in the same window (limit 2)
- **AND** the first instance Sleeps then the second instance Sleeps
- **AND** Take is called on the first instance at the start of the next window (previous-window weight 1)
- **THEN** that Take returns `Allowed` false
- **AND** `Estimated` is 3

#### Scenario: Usage is the sliding estimate
- **WHEN** Take returns
- **THEN** `Estimated` is the sliding estimate after this Take as a float
- **AND** it is not an integer ceiling of the estimate
- **AND** the Takes that still fit are reported separately as `Remaining`, not as `Estimated`

#### Scenario: Fractional window is rejected
- **WHEN** Take is called with a window of 1500ms
- **THEN** Take returns a non-nil error
- **AND** MUST NOT increment a Redis window key
- **WHEN** Take is called with a window of 500ms
- **THEN** Take returns a non-nil error

### Requirement: Peek observes without increment
`Peek(ctx, key, limit, window)` SHALL return a `Status` and an error using the same arguments, formula, and Redis keys as Take, and MUST NOT increment the current-window counter. `Status.Allowed` and `Status.Estimated` SHALL be Peek's occupancy result and sliding estimate. `Allow` SHALL remain the same operation as Take and MUST NOT become Peek. The first argument SHALL be a `context.Context`.

#### Scenario: N Peeks then Take sees count 1
- **WHEN** the window has no prior hits
- **AND** Peek is called N times for the same key, limit, and window
- **THEN** each Peek returns `Allowed` true and `Estimated` does not grow from those Peeks
- **WHEN** Take is then called once for that key
- **THEN** `Estimated` after that Take equals one hit, not N plus one
