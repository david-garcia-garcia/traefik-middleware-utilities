# Spec Delta

## ADDED Requirements

### Requirement: Concurrent first requests share one in-process bucket
The in-process store SHALL hold the mutex that guards the bucket map across lookup, create, and consume of one Allow, so concurrent first Allows on a new opaque key share one bucket.

#### Scenario: Concurrent first requests admit at most burst
- **WHEN** 16 goroutines call Allow at the same instant on a new opaque key
- **AND** rate is 1 per minute, burst is 5, and maxDelay is 500 ms
- **THEN** at most 5 of those Allows return `Allowed` true
- **AND** the store MUST NOT create a second full bucket for that key
- **AND** a non-nil Allow error fails the check
