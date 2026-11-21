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
`create`. If Sleep panics, the table SHALL NOT park the value asleep for reclaim. It SHALL end
this incarnation. Close, unmap, and release waiters SHALL all happen; their order SHALL follow
the stored `Hooks.EnforceCloseBeforeOpen` of that incarnation, not a later `Open`'s argument.
When that field is true, the table SHALL keep the key stored across Close, then unmap and
release waiters after Close returns or after a recovered Close panic. When that field is false
(the zero value), the table SHALL unmap and release waiters, then Close. Those waiters SHALL
create a new incarnation. The table SHALL NOT emit `reclaim_orphan` (Sleep did not return). It
SHALL still run Close and emit `reclaim_dispose` after Close returns or after a recovered Close
panic. This Sleep-panic ending SHALL NOT depend on grace.

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

#### Scenario: Sleep panic with enforced close waits until Close returns
- **WHEN** the last holder for a key is Done
- **AND** Sleep panics
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** Close of that incarnation is still in flight
- **AND** `Open` is called for that key
- **THEN** `create` does not run until Close has returned
- **AND** that later `Open` is free to create after Close returns

### Requirement: A caller never receives a sleeping value
`Open` SHALL NOT return a stored value before `wake` has returned for it. An `Open` that arrives
while another `Open`, a `sleep`, or a `create` is in flight for the same key SHALL wait for that
transition to finish before it decides what to do. `wake` SHALL NOT return an error when it
returns normally: the table offers no resume-failure path and never falls back to `create` when
a value cannot resume. A caller that cannot guarantee resume SHALL NOT pass Sleep and Wake hooks.
If Wake panics, that is a broken hook, not a resume failure: `Open` SHALL return an error wrapping
the panic (`fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)`), SHALL NOT return the
stored pointer, and SHALL end the incarnation. Close and unmap SHALL both happen; their order
SHALL follow the stored `Hooks.EnforceCloseBeforeOpen` of that incarnation. When that field is
true, the table SHALL keep the key stored across Close, then unmap after Close returns or after
a recovered Close panic. When that field is false (the zero value), the table SHALL unmap, then
Close. Concurrent waiters on that transition SHALL receive the same error. The wrapping error
SHALL be stored as that transition's failure before Close starts, so waiters still receive it
after waiters are released. A later `Open` SHALL be free to create.

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

#### Scenario: Wake panic with enforced close waits until Close returns
- **WHEN** a key holds a sleeping value whose Wake panics
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** Close of that incarnation is still in flight
- **AND** a later `Open` is called for that key
- **THEN** the reclaiming `Open` returns the wrapping Wake error and not the stored pointer
- **AND** concurrent waiters on that transition receive the same error
- **AND** `create` of the later `Open` does not run until Close has returned

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

### Requirement: Lifecycle events are optional Hooks passed to Open
`sleep`, `wake`, and `close` SHALL be carried by optional `func()` fields on a `Hooks` value
(`Sleep`, `Wake`, `Close`). A nil field SHALL skip that event. A caller MAY pass any subset.
The table MUST NOT discover those events by type-switching or asserting the stored `any`.

On `Open`, the caller SHALL pass that `Hooks` value beside `create`. `Open`'s `create` SHALL keep
the signature `func() (any, error)` and SHALL take no arguments.

On `OpenWithHooks` and `OpenTyped`, `create` SHALL take no arguments and SHALL return
`(any, Hooks, error)`. The `Hooks` that `create` returns SHALL be the hooks stored on that
incarnation.

`Hooks.EnforceCloseBeforeOpen` SHALL be a bool on that same value. The zero value is false: the
table unmaps the key before Close. When true, the table keeps the key mapped until Close
returns. The table SHALL read that field from the stored hooks of the incarnation that is
ending, never from a later `Open`, `OpenWithHooks`, or `OpenTyped` argument.

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

#### Scenario: OpenWithHooks stores hooks create returns
- **WHEN** `OpenWithHooks` is called with a `create` that returns a value and `Hooks` with `EnforceCloseBeforeOpen` set
- **THEN** the table stores those returned hooks on that incarnation
- **AND** a later `Open` for that key binds the same value
- **AND** that later `Open` does not run `create`

### Requirement: Slot stores Hooks at put
The table SHALL store the `Hooks` from the call that creates the incarnation (`Open`,
`OpenWithHooks`, or `OpenTyped`), including `EnforceCloseBeforeOpen`. A later `Open`,
`OpenWithHooks`, or `OpenTyped` for that key (bind or reclaim) SHALL use the stored funcs and
that stored flag and MUST NOT replace them with the argument from that later call.

#### Scenario: Later Open does not replace incarnation hooks
- **WHEN** a key is created with Sleep, Wake, and Close funcs
- **AND** a later `Open` for that key passes different or nil hooks
- **THEN** sleep, wake, and close still run the funcs stored at put

### Requirement: Interpreter tests observe Hooks
Tests that import Yaegi v0.16.1 SHALL prove three facts against interpreted code: a type-switch
to a Sleep method on a `create func() (any, error)` return does not match; `Hooks` funcs passed
to `Open` run for sleep, wake, and close; and `OpenTyped` invoked as a call expression in a
package that can name `T` returns the stored value as that type. Those tests MUST NOT start
Traefik. They MUST NOT declare a package-level var, type alias, or struct field whose type names
a generic instantiation from another package.

#### Scenario: Yaegi type-switch on create any does not match
- **WHEN** interpreted code returns a value with Sleep through `func() (any, error)`
- **AND** the test type-switches that `any` to a Sleep interface
- **THEN** the switch does not match

#### Scenario: Yaegi Open Hooks run
- **WHEN** interpreted code calls `Open` with `Hooks` that count sleep, wake, and close
- **AND** the key is orphaned, reopened within grace, then disposed
- **THEN** each of those counts is at least one

#### Scenario: Yaegi OpenTyped call expression returns T
- **WHEN** interpreted code calls `OpenTyped` as a call expression for a named `T`
- **THEN** the call returns that `T`
- **AND** the process does not panic with `nodeType2` or a type-not-found error

### Requirement: OpenTyped returns the stored value as T
`OpenTyped` SHALL call the same create-once path as `OpenWithHooks` and SHALL return the stored
value already typed as `T`. On a type mismatch it SHALL return the zero `T` and an error of the
form `reclaim: open %q: want %T, got %T`. Two successful `OpenTyped` calls for the same live key
SHALL return the same value (singleton identity). `Table` SHALL remain non-generic.

#### Scenario: OpenTyped returns a typed value
- **WHEN** `OpenTyped` is called with a `create` that returns a `*life` stored as `any`
- **THEN** the call returns that `*life` without a caller type assert
- **AND** `create` ran once

#### Scenario: OpenTyped keeps singleton identity
- **WHEN** `OpenTyped` stores a value for a key
- **AND** a second `OpenTyped` for that key runs while the incarnation is live
- **THEN** both calls return the same value
- **AND** `create` ran once

#### Scenario: OpenTyped type mismatch returns zero T
- **WHEN** `OpenTyped` is instantiated for one type
- **AND** the stored value is a different type
- **THEN** the call returns the zero value of the requested type
- **AND** the error matches `reclaim: open %q: want %T, got %T`

### Requirement: Open nil-argument errors keep table then logger then create
`Open` SHALL keep its signature and SHALL report a nil table before a nil logger before a nil
`create`. A call with a nil `create` SHALL still report a nil table or nil logger first when
those are also nil. A nil-`create` call MUST NOT start returning a different error than it does
on dest. `OpenWithHooks` SHALL use the same three error strings and the same order for its own
nil table, nil logger, and nil `create`.

#### Scenario: Open nil create reports nil table first
- **WHEN** `Open` is called on a nil table with a nil `create`
- **THEN** the error is `reclaim: open %q: nil table`

#### Scenario: Open nil create reports nil logger before nil create
- **WHEN** `Open` is called on a live table with a nil logger and a nil `create`
- **THEN** the error is `reclaim: open %q: nil logger`

#### Scenario: Open nil create on a valid table
- **WHEN** `Open` is called on a live table with a non-nil logger and a nil `create`
- **THEN** the error is `reclaim: create %q: nil create`

### Requirement: Close panic does not crash the process
If Close panics, the table SHALL recover so a production `AfterFunc` goroutine SHALL NOT kill the
process. `ready` SHALL already have been closed before Close runs when `Hooks.EnforceCloseBeforeOpen`
is false (the zero value). When that field is true, `ready` SHALL stay open across Close and SHALL
be closed after Close returns or after a recovered Close panic. A shared hook runner MAY perform
the recover, provided it hands the panic value back to its caller and that caller decides the
outcome; no runner SHALL swallow a panic as a silent success, and the table SHALL NOT continue as
if a panicking hook succeeded. Every recovered panic SHALL be surfaced exactly once and SHALL NOT
be discarded: where the recovering path has a caller to answer (`create`, Wake), as that call's
returned error; where it has none (Sleep, Close), on a `reclaim_hook_panic` line at error level
carrying the key, which hook panicked, and the panic value. Recovering a panic SHALL NOT cost the
operator the diagnostic the crash would have given them. It SHALL NOT re-panic after unsticking.

#### Scenario: AfterFunc Sleep panic does not kill the process
- **WHEN** a cancellable holder goes Done
- **AND** Sleep panics on the production AfterFunc path
- **THEN** the process continues
- **AND** a later `Open` for that key is free to create

#### Scenario: Close panic after Sleep does not kill the process
- **WHEN** Close panics while ending an incarnation
- **THEN** the process continues
- **AND** waiters are not left on an open `ready`
