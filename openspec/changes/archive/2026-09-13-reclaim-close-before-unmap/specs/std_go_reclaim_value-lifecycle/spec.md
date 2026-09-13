## MODIFIED Requirements

### Requirement: A caller never receives a sleeping value
`Open` SHALL NOT return a stored value before `wake` has returned for it. An `Open` that arrives while another `Open`, a `sleep`, a `create`, or a `close` is in flight for the same key SHALL wait for that transition to finish before it decides what to do. `wake` SHALL NOT be able to fail: the table
offers no error path for it and never falls back to `create` when a value cannot resume. A caller
that cannot guarantee resume SHALL NOT pass Sleep and Wake hooks.

#### Scenario: Open returns only after wake returned
- **WHEN** a key holds a sleeping value whose `wake` blocks
- **AND** `Open` is called for that key
- **THEN** `Open` does not return until `wake` has returned
- **AND** the value it returns is awake

#### Scenario: Concurrent Opens on a waking value all wait
- **WHEN** several `Open` calls arrive for the same sleeping key at once
- **THEN** `wake` runs once
- **AND** every caller receives the same awake value

#### Scenario: Open waits until close returns before creating
- **WHEN** the last holder for a key is Done and `close` is in flight
- **AND** `Open` is called for that key
- **THEN** `create` does not run until `close` has returned
- **AND** the value returned is a new incarnation, not the one that was closing

### Requirement: Close is always preceded by sleep
Every ending path SHALL call `sleep` before `close`, so `close` never has to handle the live
state and cleanup is not duplicated between the two. That holds when grace expires on a value
that is already sleeping (it SHALL NOT be slept twice), when the table is reset while the
incarnation is live (sleep, then close), and when grace is zero (create, sleep, close back to
back). At zero grace the value SHALL NOT be kept as a sleeping value, because there is no window in
which a sleeping value could be woken. The key SHALL remain stored until `close` has returned,
then SHALL NOT be stored. Unmapping the key before `close` returns MUST NOT happen on the
zero-grace ending path or when grace elapses.

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
- **AND** the key stayed stored until `close` returned
