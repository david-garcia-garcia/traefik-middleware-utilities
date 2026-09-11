## Purpose

Leaky-bucket pour: admit or deny a pour of water on an opaque key against leak rate and capacity. Callers own identity; the library never reads HTTP.

## Requirements

### Requirement: Add and Take pour water
`Add(key, n)` SHALL pour `n` units of water for the opaque `key` and return whether the pour is allowed, the water level after leak (and after the pour when allowed), and the duration until there is room for one more unit. `Take(key)` SHALL be Add with `n` equal to 1. When `water + n` would exceed `capacity`, Add SHALL return allowed false, SHALL NOT pour, and SHALL leave water as the leaked amount. Prefixing of `key` is the caller's job. The library MUST NOT sleep, return HTTP status, or read client address, user, tenant, or Host.

#### Scenario: Pour to capacity then deny
- **WHEN** a key starts empty
- **AND** Add is called with `n` equal to `capacity`
- **THEN** that Add returns allowed true and level equal to `capacity`
- **WHEN** Take is called once more for that key before any leak time
- **THEN** Take returns allowed false
- **AND** the stored water is still `capacity`

#### Scenario: Idle drains water
- **WHEN** a key is at `capacity`
- **AND** `capacity / leak` seconds pass with no pours
- **THEN** Level returns water 0
- **AND** the next Take returns allowed true

#### Scenario: Caller owns the key
- **WHEN** Add or Take is called with an opaque key
- **THEN** the library MUST NOT read HTTP headers, client address, user, tenant, or Host to build that key

### Requirement: Level does not pour
`Level(key)` SHALL apply leak to now and return the current water. It MUST NOT add water. Time-until-not-full is not a Level return.

#### Scenario: Level after delta t
- **WHEN** a key has water `capacity`
- **AND** half of `capacity / leak` seconds pass
- **AND** Level is called
- **THEN** Level returns about `capacity / 2`
- **AND** a following Take still sees that leaked water (Level did not pour)

### Requirement: Memory store uses the water clock
The in-memory store SHALL use `water(t) = max(0, water(t0) + poured − leak × Δt)` with Unix-microsecond elapsed, leak in water per second, and float64 water. A missing or expired key SHALL start at water 0 (empty = room up to `capacity`). Entries SHALL expire after the caller `ttl` on a later call (lazy). Construction SHALL fail when `leak <= 0`, `capacity <= 0`, or `ttl < 1s`. `Add` with `n < 1` SHALL return an error and MUST NOT pour.

#### Scenario: New rejects invalid clock
- **WHEN** NewMemory is called with leak 0 or a negative leak
- **THEN** construction returns an error
- **AND** no map entry is created

#### Scenario: Empty bucket bursts to capacity
- **WHEN** a new key has no prior water
- **AND** Add is called with `n` equal to `capacity`
- **THEN** that Add returns allowed true
- **AND** Level then returns `capacity`

### Requirement: Until-not-full is the wait for one more unit
When water after leak (and after an allowed pour) already has room for one more unit (`water + 1 <= capacity`), until-not-full SHALL be zero. Otherwise it SHALL be the time until leak brings water to `capacity - 1`.

#### Scenario: Full bucket reports positive until-not-full
- **WHEN** a key is at `capacity` after an allowed Add
- **THEN** until-not-full is greater than zero
- **AND** equals the time to leak one unit at `leak`

### Requirement: Unit and Yaegi prove pour
Compiled tests SHALL prove pour-to-cap then deny, idle drain, and Level after Δt on the memory store, and Eval encoding on a fake TCP Redis, without Traefik. Interpreted tests SHALL run the same Take scenarios with stdlib only and `useunsafe` false. A test-only clock setter SHALL exist so sequences do not wait real time.

#### Scenario: Yaegi Take on fake Redis
- **WHEN** the Yaegi GOPATH interp loads non-test `leakybucket` and `simpleredis` sources
- **AND** the probe calls Take against the compiled fake
- **THEN** the probe result is success
- **AND** the interp MUST NOT enable unsafe
