## MODIFIED Requirements

### Requirement: Peek agrees with Take before the increment
For the same clock, key, limit, and window, Peek's estimated SHALL be the already-used sliding occupancy (no hit added). Peek's `allowed` SHALL be true when that occupancy is less than or equal to `limit`. Those values SHALL match what Take would return for that same state before Take increments. Peek MUST NOT compare occupancy plus one.

#### Scenario: Peek then Take when the increment does not cross limit
- **WHEN** the clock is held fixed
- **AND** occupancy is below `limit`
- **AND** Peek is called on a key
- **AND** Take is then called on that same key, limit, and window
- **THEN** Take's allowed matches Peek's allowed
- **AND** Take's estimated equals Peek's estimated plus the one new hit's contribution

#### Scenario: Occupancy at exactly limit
- **WHEN** the clock is held fixed
- **AND** Take has been called `limit` times for that key in the window
- **AND** Peek is then called
- **THEN** Peek returns allowed true and estimated equal to `limit`
- **WHEN** Take is then called once more
- **THEN** that Take returns allowed false and estimated equal to `limit` plus one
