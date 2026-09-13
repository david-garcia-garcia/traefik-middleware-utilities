## MODIFIED Requirements

### Requirement: Sleep keeps the value stored and keeps its identity
When the last holder's context is Done, the table SHALL call the Sleep hook when that func is
non-nil and SHALL keep that value stored for the grace period. The value SHALL keep its identity:
an `Open` during that period SHALL return the same value, not a new one, and SHALL NOT run
`create`. If Sleep panics, the table SHALL NOT park the value asleep for reclaim. It SHALL end
this incarnation: Close, unmap, and release waiters. Those waiters SHALL create a new
incarnation. The table SHALL NOT emit `reclaim_orphan` (Sleep did not return). It SHALL still
run Close and emit `reclaim_dispose` after Close returns or after a recovered Close panic.

#### Scenario: A sleeping value keeps its identity
- **WHEN** every holder for a key is Done and the value has been slept
- **AND** the key is opened again before grace ends
- **THEN** the same value is returned
- **AND** `create` does not run again

#### Scenario: Sleep panic ends the incarnation
- **WHEN** the last holder for a key is Done
- **AND** Sleep panics
- **THEN** Close of that incarnation runs
- **AND** a later `Open` for that key is free to create
- **AND** that later `Open` does not hang waiting on the panicked Sleep

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

## ADDED Requirements

### Requirement: Close panic does not crash the process
If Close panics, the table SHALL recover so a production `AfterFunc` goroutine SHALL NOT kill the
process. `ready` SHALL already have been closed before Close runs. The table SHALL NOT recover
inside the Sleep, Wake, or Close runners as a silent swallow that continues as if the hook
succeeded. It SHALL NOT re-panic after unsticking.

#### Scenario: AfterFunc Sleep panic does not kill the process
- **WHEN** a cancellable holder goes Done
- **AND** Sleep panics on the production AfterFunc path
- **THEN** the process continues
- **AND** a later `Open` for that key is free to create

#### Scenario: Close panic after Sleep does not kill the process
- **WHEN** Close panics while ending an incarnation
- **THEN** the process continues
- **AND** waiters are not left on an open `ready`
