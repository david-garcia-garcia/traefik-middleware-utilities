# Spec Delta

## ADDED Requirements

### Requirement: Memory key space is the instance
Each `*Memory` SHALL be its own key space. `Allow` on one value MUST NOT consume tokens stored on a different `*Memory`. Two values that receive the same opaque `key` SHALL each admit up to `burst`. The library MUST NOT keep a process-wide Memory map or key Memory by middleware name.

#### Scenario: Shared Memory admits burst in total
- **WHEN** two callers share one `*Memory`
- **AND** they `Allow` the same opaque `key` with `maxDelay` 0 until that store denies
- **THEN** the number of `Allowed` true results across both callers is `burst`

#### Scenario: Separate Memory instances admit twice burst
- **WHEN** two `*Memory` values are constructed separately
- **AND** each `Allow`s the same opaque `key` with `maxDelay` 0 until that value denies
- **THEN** the number of `Allowed` true results across both values is `2 * burst`

### Requirement: Redis instances share one key
When the Redis store is used, bucket state lives on the server. Any `*Redis` that `Allow`s the same opaque `key` SHALL share that budget and admit `burst` in total across those instances.

#### Scenario: Two Redis limiters on one key admit burst in total
- **WHEN** two `*Redis` values `Allow` the same opaque `key` on a live engine with `maxDelay` 0 until that key denies
- **THEN** the number of `Allowed` true results across both values is `burst`
