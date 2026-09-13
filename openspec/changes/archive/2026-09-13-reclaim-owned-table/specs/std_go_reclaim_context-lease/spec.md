## ADDED Requirements

### Requirement: Caller constructs and owns a table
`reclaim` SHALL NOT expose a process-wide table. Callers SHALL create a table with `New(Config)`
and hold that instance for as long as they need shared incarnations. `New` SHALL copy
`Config.Grace` onto the table. After `New` returns, that table's grace MUST NOT change.
Independent keys on one table MUST NOT share an incarnation. Callers in other packages SHALL
type-assert the value `Open` returns. Two distinct tables MUST NOT share an incarnation even when
they are opened with the same key.

#### Scenario: Two Opens on one table share one incarnation
- **WHEN** two `Open` calls for the same key run on one table
- **THEN** both return the same stored value
- **AND** `create` runs once

#### Scenario: Two tables do not share
- **WHEN** two tables are each opened for the same key
- **THEN** each table stores its own value
- **AND** `create` runs once per table

#### Scenario: Constructor config is fixed
- **WHEN** a table is created with `New(Config)`
- **AND** the caller later writes a different `Grace` on that `Config` value
- **THEN** the table's grace is still the value `New` copied

### Requirement: New copies grace from Config
`New(Config)` SHALL be the only public constructor. A negative `Grace` SHALL become the product
default of 10 seconds (`DefaultGrace`). A zero `Grace` SHALL stay zero. `NewTable`, `Default`,
package `Open`, package `Reset`, and package `ResetWith` SHALL NOT exist.

#### Scenario: Package singleton APIs are absent
- **WHEN** the `reclaim` package is listed for exported constructors and package funcs
- **THEN** there is no `Default`, package `Open`, `Reset`, `ResetWith`, or `NewTable`

## MODIFIED Requirements

### Requirement: Grace is configurable
Grace SHALL be how long a **sleeping** value is kept before it is disposed. Because a sleeping
value has released what is expensive to hold idle, a long grace is cheap: the reason to keep it
long is that a sleeping value costs little, not that reloads are fast. The table SHALL use the
grace `New` copied from `Config`. A zero grace SHALL dispose of the value as soon as the last
holder is gone, with no sleeping window at all. A negative grace SHALL become the product default
of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`).

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** the value is slept and disposed without waiting
- **AND** the key is not left stored

### Requirement: Library Open loads under Traefik Yaegi
A Traefik local plugin SHALL import this module's `reclaim` package, hold one table created with
`New(Config)`, and call that table's `Open` from `New` with `Hooks` that log sleep, wake, and
close. Traefik SHALL start. A request through that plugin SHALL succeed. Two plugin instances that
Open the same key on that table SHALL receive the same stored value. Those hooks SHALL run under
Yaegi. Inert hooks MUST NOT be accepted as success for this load.

#### Scenario: Fake plugin starts and shares one incarnation
- **WHEN** Traefik v3.7.11 loads a local plugin whose `New` calls `Open` on a caller-owned table for a shared key
- **AND** two routes each construct that plugin
- **THEN** Traefik's API is reachable
- **AND** a request through each route succeeds
- **AND** both instances observe the same stored value identity
- **AND** Traefik logs include `reclaim_put` and `reclaim_bind`

#### Scenario: Reload runs sleep then wake hooks
- **WHEN** both plugin instances for that shared key are torn down and constructed again within grace
- **THEN** Traefik logs include `reclaim_orphan` and `reclaim_reclaim`
- **AND** Traefik logs include the plugin's sleep hook line and wake hook line

#### Scenario: Teardown runs the close hook
- **WHEN** every plugin instance for that shared key is torn down and grace elapses
- **THEN** Traefik logs include `reclaim_dispose`
- **AND** Traefik logs include the plugin's close hook line

## REMOVED Requirements

### Requirement: Process table is a singleton
**Reason**: Callers need to own table lifetime and constructor config. A process-wide table is always `DefaultGrace` and shares keys across unrelated plugins.
**Migration**: Call `New(Config)` and keep the `*Table`. Production sharing is a package-level table in the caller. Use `table.Open` instead of `reclaim.Open`. Use `Table.Reset` instead of package `Reset` / `ResetWith`.
