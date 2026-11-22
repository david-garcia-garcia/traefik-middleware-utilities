# Spec Delta

## MODIFIED Requirements

### Requirement: OpenTyped returns the stored value as T
`OpenTyped` SHALL call the same create-once path as `OpenWithHooks` and SHALL return the stored
value already typed as `T`. On a type mismatch it SHALL return the zero `T` and an error of the
form `reclaim: open %q: want %T, got %T`, and it SHALL NOT keep the caller as a holder: the
holder count SHALL be as if that call had not bound, its context SHALL no longer keep the value
awake, and when no other holder remains the table SHALL end or park the incarnation as for any
last holder leaving (Close runs at grace 0). A mismatch SHALL NOT undo a `create` that already
ran. The holder of a mismatched call SHALL be dropped exactly once, even when its context ends
later. Two successful `OpenTyped` calls for the same live key SHALL return the same value
(singleton identity). `Table` SHALL remain non-generic.

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

#### Scenario: OpenTyped type mismatch does not keep a holder
- **WHEN** the table grace is zero
- **AND** `OpenTyped` creates a value of a different type than requested
- **AND** the caller's context is still live
- **THEN** Close runs once for that incarnation
- **AND** a later end of the caller's context does not drop a holder again
