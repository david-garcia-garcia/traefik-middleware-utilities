## MODIFIED Requirements

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

## ADDED Requirements

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
