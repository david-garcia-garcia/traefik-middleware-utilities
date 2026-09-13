## MODIFIED Requirements

### Requirement: Sliding estimate uses current and previous windows
The current window start SHALL be `floor(unixSeconds / windowSeconds) × windowSeconds`. Redis keys SHALL be `{opaqueKey}:{windowStart}` and `{opaqueKey}:{previousWindowStart}`. A missing previous window SHALL count as zero. Window length SHALL be a whole number of seconds. Sub-second windows MUST NOT be supported.

#### Scenario: Dump at the window boundary does not double the limit
- **WHEN** a key has used its full limit near the end of a window
- **AND** Take is called at the start of the next window
- **THEN** the estimate still includes the previous window's hits
- **AND** the new window MUST NOT admit a second full limit the way a fixed window would

#### Scenario: Buffered two-client dump at the window boundary
- **WHEN** two limiter instances with `sync_rate` greater than zero each record one hit in the same window (limit 2)
- **AND** the first instance Sleeps then the second instance Sleeps
- **AND** Take is called on the first instance at the start of the next window (previous-window weight 1)
- **THEN** that Take returns allowed false
- **AND** the sliding estimate is 3

#### Scenario: Usage is the sliding estimate
- **WHEN** Take returns
- **THEN** the usage value is the sliding estimate after this Take as a float
- **AND** it is not remaining quota and not an integer ceiling of the estimate

### Requirement: Peek agrees with Take before the increment
For the same clock, key, limit, and window, Peek's `allowed` and estimated SHALL match the values Take would return for that same state before Take increments, except when buffered mode (`sync_rate` greater than zero) still holds a previous-window key from this instance's last flush while Redis has a higher shared count: then Peek MAY report the stale previous until Take GETs it. Peek MUST NOT GET previous on every call to close that gap.

#### Scenario: Peek then Take at a frozen clock
- **WHEN** the clock is held fixed
- **AND** Peek is called on a key
- **AND** Take is then called on that same key, limit, and window
- **THEN** Take's allowed matches Peek's allowed
- **AND** Take's estimated equals Peek's estimated plus the one new hit's contribution

#### Scenario: Buffered Peek may lag previous until Take
- **WHEN** `sync_rate` is greater than zero
- **AND** another instance has flushed a higher shared count onto a key that is now previous
- **AND** Peek is called on this instance at that clock
- **THEN** Peek MAY still use this instance's last flush return for previous
- **AND** the next Take SHALL GET previous when `local_delta` is 0
- **AND** Peek MUST NOT GET previous solely because `local_delta` is 0
