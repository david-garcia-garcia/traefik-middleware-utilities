# Spec Delta

## ADDED Requirements

### Requirement: Concurrent first requests share one Eval
Sixteen simultaneous first Allows on a new Redis or Dragonfly key with rate 1 per minute, burst 5, and maxDelay 500 ms SHALL admit at most burst, because one Eval owns HGETALL, HSET, and EXPIRE for that consume.

#### Scenario: Concurrent first requests admit at most burst
- **WHEN** 16 goroutines call Allow at the same instant on a new opaque Redis or Dragonfly key
- **AND** rate is 1 per minute, burst is 5, and maxDelay is 500 ms
- **THEN** at most 5 of those Allows return `Allowed` true
- **AND** Go MUST NOT GET or SET that hash for the bucket
- **AND** a non-nil Allow error fails the check
