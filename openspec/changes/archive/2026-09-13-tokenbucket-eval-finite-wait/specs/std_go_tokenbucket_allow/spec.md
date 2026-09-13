## ADDED Requirements

### Requirement: Eval wait that is not a finite number is not a consume
When the Redis store is used and Eval returns three fields, Allow SHALL treat the wait field as a finite number of microseconds. A wait that is not a number, including `nan`, `+Inf`, `-Inf`, and `inf`, SHALL return the existing wait-is-not-a-number error (`tokenbucket: eval wait is not a number`). Allow MUST NOT return allowed true with a nil error. Allow MUST NOT return allowed false with a nil error. Allow MUST NOT introduce a second error for that wait. Admit and refund comparisons MUST NOT replace this rule: a non-finite wait is not a delay that can be compared to maxDelay.

#### Scenario: Non-finite wait is wait-is-not-a-number
- **WHEN** Eval returns three fields whose wait is `nan`, `+Inf`, `-Inf`, or `inf`
- **THEN** Allow returns `tokenbucket: eval wait is not a number`
- **AND** does not return allowed true
- **AND** does not return a nil error
