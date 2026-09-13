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
MUST NOT happen on that incarnation's zero-grace ending path or when grace elapses. When that
field is false (the zero value), the table SHALL unmap the key before Close, so a concurrent
`Open` MAY create while Close is in flight. Tests-only `Reset` MAY unmap first regardless of
the field.

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

#### Scenario: Interpreted grace expire still closes
- **WHEN** a table with positive grace runs interpreted under Yaegi v0.16.1
- **AND** the last holder for several keys is Done
- **AND** no `Open` arrives during grace
- **THEN** each incarnation is disposed
- **AND** Close of each incarnation runs
