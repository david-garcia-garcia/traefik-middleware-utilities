## MODIFIED Requirements

### Requirement: Open creates once and binds a context
`Open(ctx, key, logger, create, hooks)` SHALL create the value on the first call for a key, store
it, store `hooks` on that incarnation, and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is
nil. `Open` SHALL return an error if `logger` is nil. The table MUST NOT keep a logger of its own;
`logger` is the only logger for that Open. A holder whose `Done` is nil (`context.Background`)
SHALL be treated as live until `ctx.Err()` is set. `create` SHALL take no arguments (Yaegi cannot
call `func(context.Context) (any, error)`). The table SHALL register the key before it runs
`create`, so concurrent first `Open` calls for one key run `create` exactly once and every caller
receives that one value; no `Open` SHALL create a value that is then discarded. If `create`
returns an error the key SHALL NOT be stored, every caller waiting on that create SHALL receive
that error, and a later `Open` SHALL be free to try again. A later `Open` for the same key (live
or sleeping) SHALL return the stored value, bind the new context, and MUST NOT run `create`. Two
live contexts on one key SHALL keep the value until both are Done. A stale holder drop from a
previous incarnation or from `Reset` MUST NOT change a later incarnation of the same key.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** the incarnation does not end while either context is not Done

#### Scenario: Second create dispose is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again
- **THEN** `create` does not run
- **AND** the stored value is the one returned

#### Scenario: Concurrent first Opens run create once
- **WHEN** two first `Open` calls for the same key race
- **THEN** `create` runs exactly once
- **AND** both calls return that one value
- **AND** no value is created and then discarded

#### Scenario: Create error reaches every waiting caller
- **WHEN** two first `Open` calls for the same key race and `create` returns an error
- **THEN** both calls return that error
- **AND** nothing is stored for that key
- **AND** a later `Open` for that key runs `create` again

#### Scenario: Missing context panics
- **WHEN** `Open` is called with a nil context
- **THEN** `Open` panics

#### Scenario: Nil logger is rejected
- **WHEN** `Open` is called with a nil logger
- **THEN** `Open` returns an error
- **AND** no incarnation is stored

#### Scenario: Nil-Done holder stays live until Err is set
- **WHEN** `Open` binds a holder whose `Done` is nil
- **AND** that holder has not set `Err`
- **THEN** the incarnation stays live
- **WHEN** that holder then sets `Err` without closing a `Done` channel
- **THEN** the table drops that holder

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while sleeping, `Reset`, or a zero-grace drop), the table
SHALL call the Close hook when that func is non-nil and SHALL wait until it has returned before
it emits `reclaim_dispose`. The Close hook SHALL be called at most once per incarnation. Because
the table waits, a Close hook that blocks blocks whoever ended the incarnation; Close hooks
SHALL NOT block. Every goroutine the table starts for a key SHALL exit once that key's holder
contexts are Done and its incarnation has ended.

#### Scenario: Dispose log implies Close has returned
- **WHEN** a key is orphaned and grace elapses
- **AND** the Close hook is set
- **THEN** that func has returned before `reclaim_dispose` is emitted for that key
- **AND** that func did not observe a dispose line already written for that key

#### Scenario: Reset closes the value before it reports dispose
- **WHEN** `Reset` is called on a table that still has an incarnation
- **AND** the Close hook is set
- **THEN** that func has returned before `reclaim_dispose` is emitted for that key

#### Scenario: Cancellable holders do not park a waiter for the hold
- **WHEN** many keys are opened with holder contexts whose `Done` is non-nil
- **AND** those contexts are still live
- **THEN** the table owns no more goroutines than it did before those `Open` calls

#### Scenario: Goroutines do not outlive the incarnation
- **WHEN** many keys are opened, then every holder context is Done and every incarnation has ended
- **THEN** the table owns no more goroutines than it did before those `Open` calls
