## ADDED Requirements

### Requirement: GET miss matches through wrapping
Previous-window and exact GET misses SHALL be classified with `IsMiss` (or `errors.Is` against the exported miss sentinel). A miss whose `Error()` text is no longer exactly `redis:miss` because it was wrapped SHALL still count as zero. The limiter MUST NOT match miss by `err.Error() ==` the miss token.

#### Scenario: Wrapped miss counts as zero
- **WHEN** GET would return a miss wrapped with `%w`
- **THEN** the limiter treats that counter as zero
- **AND** it does not return a hard error
