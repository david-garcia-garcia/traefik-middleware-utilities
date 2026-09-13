## MODIFIED Requirements

### Requirement: Sliding estimate uses current and previous windows
The current window start SHALL be `floor(unixSeconds / windowSeconds) × windowSeconds`. Redis keys SHALL be `{opaqueKey}:{windowStart}` and `{opaqueKey}:{previousWindowStart}`. A missing previous window SHALL count as zero. Window length SHALL be a whole number of seconds. Sub-second windows MUST NOT be supported. Take and Peek MUST return an error when `window` is shorter than one second or is not an integer number of seconds (for example 1500ms). They MUST NOT accept that window and silently use truncated whole-second buckets. Weight and TTL SHALL use that whole-second length only.

#### Scenario: Dump at the window boundary does not double the limit
- **WHEN** a key has used its full limit near the end of a window
- **AND** Take is called at the start of the next window
- **THEN** the estimate still includes the previous window's hits
- **AND** the new window MUST NOT admit a second full limit the way a fixed window would

#### Scenario: Usage is the sliding estimate
- **WHEN** Take returns
- **THEN** the usage value is the sliding estimate after this Take as a float
- **AND** it is not remaining quota and not an integer ceiling of the estimate

#### Scenario: Fractional window is rejected
- **WHEN** Take is called with a window of 1500ms
- **THEN** Take returns a non-nil error
- **AND** MUST NOT increment a Redis window key
- **WHEN** Take is called with a window of 500ms
- **THEN** Take returns a non-nil error
