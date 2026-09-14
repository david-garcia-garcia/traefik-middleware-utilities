## MODIFIED Requirements

### Requirement: Close is always preceded by sleep
Every ending path SHALL call `sleep` before `close`, so `close` never has to handle the live
state and cleanup is not duplicated between the two. That holds when grace expires on a value
that is already sleeping (it SHALL NOT be slept twice), when the table is reset while the
incarnation is live (sleep, then close), and when grace is zero (create, sleep, close back to
back). At zero grace the value SHALL NOT be kept, because there is no window in which a sleeping
value could be woken.

When the ending incarnation stored `Hooks.EnforceCloseBeforeOpen`, the key SHALL remain stored
until `close` has returned, then SHALL NOT be stored. Unmapping the key before `close` returns
MUST NOT happen on that incarnation's zero-grace ending path, when grace elapses, or on a
Sleep-panic or Wake-panic ending. When that field is false (the zero value), the table SHALL
unmap the key before Close, so a concurrent `Open` MAY create while Close is in flight.
Tests-only `Reset` SHALL unmap first regardless of the field.

If Sleep panics during `Reset` of an awake value, the table SHALL NOT emit `reclaim_orphan`.
It SHALL still run Close and emit `reclaim_dispose` after Close returns or after a recovered
Close panic.

The positive-grace wait MUST be a compiled stdlib waiter (`time.AfterFunc` or equivalent). It
MUST NOT be an interpreted goroutine that `select`s on a timer channel and a wake channel. The
wait MUST NOT run on the last-holder drop caller. An `Open` that reclaims a sleeping value
SHALL cancel that waiter so expire does not dispose it. `expire` SHALL still refuse a slot that
is not asleep or that still has holders.

#### Scenario: Grace expiry does not sleep a sleeping value twice
- **WHEN** the last holder for a key is Done and grace elapses without a new `Open`
- **THEN** the value received exactly one `sleep`
- **AND** `close` ran after that `sleep`

#### Scenario: Reset on a live incarnation sleeps before it closes
- **WHEN** the table is reset while a key still has a live holder
- **THEN** that value received `sleep` and then `close`, in that order

#### Scenario: Zero grace runs create, sleep, and close back to back
- **WHEN** a table with zero grace has its last holder for a key go Done
- **THEN** the value received `sleep` and then `close`
- **AND** the key is no longer stored

#### Scenario: Enforced close keeps the key stored until close returns
- **WHEN** a table with zero grace has its last holder for a key go Done
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **THEN** the key stayed stored until `close` returned

#### Scenario: Enforced close on Sleep panic keeps the key stored until close returns
- **WHEN** Sleep panics
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **THEN** the key stayed stored until `close` returned

#### Scenario: Interpreted grace expire still closes
- **WHEN** a table with positive grace runs interpreted under Yaegi v0.16.1
- **AND** the last holder for several keys is Done
- **AND** no `Open` arrives during grace
- **THEN** each incarnation is disposed
- **AND** Close of each incarnation runs

#### Scenario: Reset Sleep panic skips orphan and still disposes
- **WHEN** `Reset` is called on a table that still has an awake incarnation
- **AND** Sleep panics
- **THEN** Close of that incarnation runs
- **AND** `reclaim_orphan` is not emitted for that key
- **AND** `reclaim_dispose` is emitted for that key
- **AND** the key is not stored after `Reset` returns

#### Scenario: Reset unmaps first even when close is enforced
- **WHEN** an incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** `Reset` is called while that incarnation is still stored
- **AND** Close of that incarnation is still in flight
- **AND** `Open` is called for that key
- **THEN** `create` runs before Close returns
- **AND** that `Open` returns a new incarnation, not the one Reset is closing
