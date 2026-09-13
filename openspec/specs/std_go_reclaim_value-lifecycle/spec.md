## Purpose

Defines the four events a value stored on the reclaim table receives — create, sleep, wake, close
— their order, the optional Hooks funcs that carry sleep, wake, and close, and the guarantee that
close is always preceded by sleep so cleanup is never written twice.

## Requirements

### Requirement: A stored value receives four lifecycle events
The table SHALL drive a stored value through `create -> (sleep -> wake)* -> sleep -> close`.
`create` SHALL run once per incarnation, on the first `Open` for a key that finds it absent, and
the value SHALL come back awake. `sleep` SHALL run when the last holder's context is Done. `wake`
SHALL run when an `Open` arrives for a stored, sleeping value. `close` SHALL run once, when the
value is dropped for good. `sleep` and `wake` are a matched, repeating pair; `create` and `close`
happen once each. A value SHALL NOT receive `wake` without a preceding `sleep`, and SHALL NOT
receive two `sleep` calls without a `wake` between them.

#### Scenario: One incarnation across several sleep and wake cycles
- **WHEN** a key is opened, orphaned, and reopened within grace several times
- **THEN** `create` ran once
- **AND** the stored value received one `sleep` and one `wake` per cycle, alternating
- **AND** every `Open` returned the same value

#### Scenario: Create runs once per incarnation
- **WHEN** a key that is absent is opened
- **THEN** `create` runs once
- **AND** the value the caller receives has not been slept

### Requirement: Sleep keeps the value stored and keeps its identity
When the last holder's context is Done, the table SHALL call the Sleep hook when that func is
non-nil and SHALL keep that value stored for the grace period. The value SHALL keep its identity:
an `Open` during that period SHALL return the same value, not a new one, and SHALL NOT run
`create`.

#### Scenario: A sleeping value keeps its identity
- **WHEN** every holder for a key is Done and the value has been slept
- **AND** the key is opened again before grace ends
- **THEN** the same value is returned
- **AND** `create` does not run again

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

### Requirement: Lifecycle events are optional Hooks passed to Open
`sleep`, `wake`, and `close` SHALL be carried by optional `func()` fields on a `Hooks` value
passed to `Open` (`Sleep`, `Wake`, `Close`). A nil field SHALL skip that event. A caller MAY
pass any subset. `create` SHALL keep the signature `func() (any, error)` and SHALL take no
arguments. The table MUST NOT discover those events by type-switching or asserting the stored
`any`.

#### Scenario: Empty Hooks still runs the full table lifecycle
- **WHEN** a value is stored with all-nil `Hooks` and later ends
- **THEN** the table completes the incarnation without error
- **AND** it still reports the end

#### Scenario: Only Close hook runs
- **WHEN** `Hooks` has only `Close` set
- **AND** its key is orphaned and grace elapses
- **THEN** `Close` is called exactly once

#### Scenario: Compiled tests observe all four events
- **WHEN** `Hooks` wires Sleep, Wake, and Close and the key is exercised with `go test`
- **THEN** create, sleep, wake, and close run in the order this spec requires

### Requirement: Slot stores Hooks at put
The table SHALL store the `Hooks` from the `Open` that creates the incarnation. A later `Open`
for that key (bind or reclaim) SHALL use the stored funcs and MUST NOT replace them with the
argument from that later call.

#### Scenario: Later Open does not replace incarnation hooks
- **WHEN** a key is created with Sleep, Wake, and Close funcs
- **AND** a later `Open` for that key passes different or nil hooks
- **THEN** sleep, wake, and close still run the funcs stored at put

### Requirement: Interpreter tests observe Hooks
Tests that import Yaegi v0.16.1 SHALL prove two facts against an interpreted `create
func() (any, error)`: a type-switch to a Sleep method on the returned `any` does not match, and
`Hooks` funcs passed to `Open` run for sleep, wake, and close. Those tests MUST NOT start Traefik.

#### Scenario: Yaegi type-switch on create any does not match
- **WHEN** interpreted code returns a value with Sleep through `func() (any, error)`
- **AND** the test type-switches that `any` to a Sleep interface
- **THEN** the switch does not match

#### Scenario: Yaegi Open Hooks run
- **WHEN** interpreted code calls `Open` with `Hooks` that count sleep, wake, and close
- **AND** the key is orphaned, reopened within grace, then disposed
- **THEN** each of those counts is at least one
