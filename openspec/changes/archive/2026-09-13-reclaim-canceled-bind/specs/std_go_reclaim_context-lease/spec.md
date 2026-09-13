## MODIFIED Requirements

### Requirement: Open creates once and binds a context
`Open(ctx, key, logger, create, hooks)` SHALL create the value on the first call for a key, store
it, store `hooks` on that incarnation, and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is
nil. `Open` SHALL return an error if `logger` is nil. The table MUST NOT keep a logger of its own;
`logger` is the only logger for that Open. A holder whose `Done` is nil (`context.Background`)
SHALL be treated as live until `ctx.Err()` is set. `create` SHALL take no arguments (Yaegi cannot
call `func(context.Context) (any, error)`). The table SHALL register the key before it runs
`create`, so concurrent first `Open` calls for one key run `create` exactly once; no `Open` SHALL
create a value that is then discarded. If `create` returns an error the key SHALL NOT be stored,
every caller waiting on that create SHALL receive that error, and a later `Open` SHALL be free
to try again. `(value, nil)` SHALL mean this call bound a holder that was still live at return.
If `ctx.Err()` is set at bind time, `Open` SHALL return `(nil, ctx.Err())` and MUST NOT give the
caller the stored pointer. After a successful create, a bind to a live incarnation, or a reclaim of
a sleeping value, if `ctx.Err()` is set, `Open` SHALL drop that holder on the same call (the value
MUST NOT leak) and return the context error; Close MAY already have run when this was the last
holder. Waiters that bound when that create finished SHALL still receive the stored value while
their own context is live. If `ctx` is still live at bind, `Open` SHALL watch that holder until
it is done. A later `Open` for the same key (live or sleeping) whose context is still live SHALL
return the stored value, bind the new context, and MUST NOT run `create`. Two live contexts on
one key SHALL keep the value until both are Done. A stale holder drop from a previous
incarnation or from `Reset` MUST NOT change a later incarnation of the same key. An `Open` that
reclaims a sleeping value SHALL still wait until wake has returned before it returns, including
when it then returns `ctx.Err()` instead of the pointer.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** the incarnation does not end while either context is not Done

#### Scenario: Second create dispose is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again with a live context
- **THEN** `create` does not run
- **AND** the stored value is the one returned

#### Scenario: Concurrent first Opens run create once
- **WHEN** two first `Open` calls for the same key race
- **THEN** `create` runs exactly once
- **AND** both calls whose context is still live at bind return that one value
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

#### Scenario: Canceled ctx during create does not receive the pointer
- **WHEN** `Open` runs `create` for a new key
- **AND** that call's holder context is Done before bind
- **AND** `create` returns a value
- **THEN** `Open` returns that context error
- **AND** `Open` does not return the stored pointer
- **AND** Close MAY have run because that holder was already done

#### Scenario: Waiter still receives the live value
- **WHEN** two first `Open` calls for the same key race
- **AND** `create` returns a value
- **AND** the creating call's context is Done at bind
- **AND** the waiting call's context is still live at bind
- **THEN** the creating call returns that context error and not the pointer
- **AND** the waiting call returns the stored value with no error

#### Scenario: Already-done ctx on a live key does not receive the pointer
- **WHEN** a key already has a live incarnation
- **AND** `Open` is called with a context whose `Err` is already set
- **THEN** `Open` returns that context error
- **AND** `Open` does not return the stored pointer
- **AND** other live holders keep the incarnation

#### Scenario: Already-done ctx on a sleeping key does not receive the pointer
- **WHEN** a key holds a sleeping value
- **AND** `Open` is called with a context whose `Err` is already set
- **THEN** wake has returned before `Open` returns
- **AND** `Open` returns that context error
- **AND** `Open` does not return the stored pointer
