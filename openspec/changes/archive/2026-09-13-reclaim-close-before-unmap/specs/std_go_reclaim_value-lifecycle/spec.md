## MODIFIED Requirements

### Requirement: A caller never receives a sleeping value
`Open` SHALL NOT return a stored value before `wake` has returned for it. An `Open` that arrives
while another `Open`, a `sleep`, or a `create` is in flight for the same key SHALL wait for that
transition to finish before it decides what to do. `wake` SHALL NOT return an error when it
returns normally: the table offers no resume-failure path and never falls back to `create` when
a value cannot resume. A caller that cannot guarantee resume SHALL NOT pass Sleep and Wake hooks.
If Wake panics, that is a broken hook, not a resume failure: `Open` SHALL return an error wrapping
the panic (`fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)`), SHALL NOT return the
stored pointer, and SHALL end the incarnation (Close, unmap). Concurrent waiters on that
transition SHALL receive the same error. A later `Open` SHALL be free to create.

When the ending incarnation stored `Hooks.EnforceCloseBeforeOpen`, an `Open` that arrives while
`close` is in flight for that key SHALL wait for Close to return (or for its panic to be
recovered) before it creates. When that field is false (the zero value), a concurrent `Open` MAY
create while Close is still in flight.

#### Scenario: Open returns only after wake returned
- **WHEN** a key holds a sleeping value whose `wake` blocks
- **AND** `Open` is called for that key
- **THEN** `Open` does not return until `wake` has returned
- **AND** the value it returns is awake

#### Scenario: Concurrent Opens on a waking value all wait
- **WHEN** several `Open` calls arrive for the same sleeping key at once
- **THEN** `wake` runs once
- **AND** every caller receives the same awake value

#### Scenario: Wake panic returns an error and unsticks the key
- **WHEN** a key holds a sleeping value whose Wake panics
- **AND** `Open` is called for that key
- **THEN** that `Open` returns an error wrapping the panic
- **AND** it does not return the stored pointer
- **AND** Close of that incarnation runs
- **AND** a later `Open` for that key is free to create

#### Scenario: Open waits until close returns before creating
- **WHEN** the last holder for a key is Done and `close` is in flight
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** `Open` is called for that key
- **THEN** `create` does not run until `close` has returned
- **AND** the value returned is a new incarnation, not the one that was closing

#### Scenario: Open creates during close by default
- **WHEN** the last holder for a key is Done and `close` is in flight
- **AND** that incarnation did not store `Hooks.EnforceCloseBeforeOpen`
- **AND** `Open` is called for that key
- **THEN** `Open` returns a new incarnation while Close is still blocked

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
