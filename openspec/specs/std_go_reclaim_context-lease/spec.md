## Purpose

Defines a keyed reclaim table that stores one value per key as `any`, survives context cancel when the same key is opened again within grace, and cancels the incarnation lifetime when it is not. The table lives in `reclaim` and is reusable across packages. Callers type-assert. Yaegi cannot instantiate `Table[T]` from another package; this table is not generic.

## Requirements

### Requirement: Table file depends only on the Go standard library
The `Table` source file SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, e2e, or vendor packages. It MUST store `any`. It MUST NOT be a generic `Table[T]` instantiated as `otherpkg.Table[*T]` (Yaegi panics or fails import).

#### Scenario: Stdlib-only imports
- **WHEN** `table.go` is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Caller constructs and owns a table
`reclaim` SHALL NOT expose a process-wide table. Callers SHALL create a table with `New(Config)`
and hold that instance for as long as they need shared incarnations. `New` SHALL copy
`Config.Grace` onto the table. After `New` returns, that table's grace MUST NOT change.
Independent keys on one table MUST NOT share an incarnation. Callers in other packages SHALL
type-assert the value `Open` returns. Two distinct tables MUST NOT share an incarnation even when
they are opened with the same key.

#### Scenario: Two Opens on one table share one incarnation
- **WHEN** two `Open` calls for the same key run on one table
- **THEN** both return the same stored value
- **AND** `create` runs once

#### Scenario: Two tables do not share
- **WHEN** two tables are each opened for the same key
- **THEN** each table stores its own value
- **AND** `create` runs once per table

#### Scenario: Constructor config is fixed
- **WHEN** a table is created with `New(Config)`
- **AND** the caller later writes a different `Grace` on that `Config` value
- **THEN** the table's grace is still the value `New` copied

### Requirement: New copies grace from Config
`New(Config)` SHALL be the only public constructor. A negative `Grace` SHALL become the product
default of 10 seconds (`DefaultGrace`). A zero `Grace` SHALL stay zero. `NewTable`, `Default`,
package `Open`, package `Reset`, and package `ResetWith` SHALL NOT exist.

#### Scenario: Package singleton APIs are absent
- **WHEN** the `reclaim` package is listed for exported constructors and package funcs
- **THEN** there is no `Default`, package `Open`, `Reset`, `ResetWith`, or `NewTable`

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

### Requirement: Cancel then open within grace does not dispose
When every bound context for a key is Done, the table SHALL put the stored value to sleep and
keep it for a grace period before it disposes of it. If the same key is opened again with a live
context before grace ends, the table SHALL wake that value and MUST NOT dispose of it. That
reclaim MUST NOT run `create` again.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new `Open` for that key occurs before grace ends
- **THEN** the incarnation is not disposed
- **AND** the new context is tracked
- **AND** the stored value is returned awake

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no `Open` for that key occurs during grace
- **THEN** the incarnation is disposed once

### Requirement: Keys are independent
Canceling the lifetime of one key MUST NOT cancel the lifetime of another key.

#### Scenario: One key times out
- **WHEN** key A’s contexts are all Done and grace elapses
- **AND** key B still has a live context
- **THEN** only key A’s lifetime is canceled

### Requirement: Grace is configurable
Grace SHALL be how long a **sleeping** value is kept before it is disposed. Because a sleeping
value has released what is expensive to hold idle, a long grace is cheap: the reason to keep it
long is that a sleeping value costs little, not that reloads are fast. The table SHALL use the
grace `New` copied from `Config`. A zero grace SHALL dispose of the value as soon as the last
holder is gone, with no sleeping window at all. A negative grace SHALL become the product default
of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`).

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** the value is slept and disposed without waiting
- **AND** the key is not left stored
- **AND** there was no window in which an `Open` could wake that value

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create),
holder attached, last holder gone and the value put to sleep (orphan), a sleeping value woken by
an `Open` (reclaim), and incarnation disposed. Each line MUST include the key. Message strings
SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`,
`reclaim_reclaim`, `reclaim_dispose`). All five messages SHALL be logged at debug. A recovered hook panic that has no caller to answer (Sleep, Close) SHALL be reported once on a separate `reclaim_hook_panic` line at error level, carrying the key, which hook panicked, and the panic value, so that recovering a panic never makes it silent. Put, bind, and
reclaim SHALL use the logger passed to the `Open` that caused them. Orphan and dispose SHALL use
the logger from the last `Open` that bound that key. `Reset` SHALL emit `reclaim_dispose` for
each disposed key using that slot's last Open logger, preceded by `reclaim_orphan` when that
incarnation was still awake. Log lines MUST NOT be emitted while the table mutex is held.

Every line SHALL mean the work it names has already happened: `reclaim_orphan` is emitted after
`sleep` has returned, `reclaim_reclaim` after `wake` has returned, and `reclaim_dispose` after
the Close hook has returned (or immediately when that func is nil, or after a recovered Close
panic was reported). For one incarnation
`reclaim_orphan` SHALL always precede the `reclaim_dispose` that ends it, at every grace duration
including zero, with no exception for `Reset`.

At zero grace the table keeps no sleeping value, so an `Open` that races the last holder going
away SHALL be logged as a new incarnation (`reclaim_put` then `reclaim_bind`), not as
`reclaim_reclaim`. When the previous incarnation stored `Hooks.EnforceCloseBeforeOpen`, that new
incarnation's `create` SHALL NOT start until Close of the previous incarnation has returned, so
`reclaim_dispose` for the previous incarnation SHALL precede `reclaim_put` of the next. When that
field is false (the zero value), `create` MAY start while Close is still in flight, and
`reclaim_dispose` NEED NOT precede `reclaim_put` of the next.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is opened, then all of A's contexts are Done
- **AND** key B is opened (new incarnation) before or after A is put to sleep
- **AND** A is not opened again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B is not disposed

#### Scenario: Orphan precedes dispose at a short grace
- **WHEN** a table with a grace shorter than a millisecond has its last holder for a key go Done
- **AND** the grace elapses and the incarnation is disposed
- **THEN** the recorded order for that key is `reclaim_orphan` then `reclaim_dispose`

#### Scenario: Zero grace open racing the drop is a plain bind
- **WHEN** a table with zero grace has its last holder for a key go Done
- **AND** an `Open` for that key races that drop
- **THEN** that `Open` records `reclaim_put` and `reclaim_bind` for the key
- **AND** it does not record `reclaim_reclaim`

#### Scenario: Enforced close dispose precedes the next put
- **WHEN** a table with zero grace has its last holder for a key go Done
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** an `Open` for that key races that drop
- **THEN** `reclaim_dispose` for the previous incarnation is recorded before that `reclaim_put`

#### Scenario: Recovered hook panic is reported
- **WHEN** a Sleep or Close hook panics and the table recovers it
- **THEN** logs include `reclaim_hook_panic` at error level for that key
- **AND** that line carries which hook panicked and the panic value

#### Scenario: Reset logs orphan then dispose
- **WHEN** `Reset` is called on a table that still has a live incarnation
- **THEN** logs include `reclaim_orphan` then `reclaim_dispose` for that key
- **AND** a later `Open` of the same key creates a new incarnation that a stale holder drop MUST NOT dispose

#### Scenario: Open logger level gates put and dispose
- **WHEN** `Open` is called with a logger whose handler level is debug
- **THEN** `reclaim_put` is emitted at debug
- **WHEN** that incarnation is later disposed
- **THEN** `reclaim_dispose` is emitted at debug
- **WHEN** `Open` is called with a logger whose handler level is info
- **THEN** `reclaim_put` and `reclaim_dispose` are not emitted

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while sleeping, `Reset`, or a zero-grace drop), the table
SHALL call the Close hook when that func is non-nil and SHALL wait until it has returned, or until
its panic has been recovered and reported, before it emits `reclaim_dispose`. The table SHALL NOT
let a Close panic escape, because Close can run on an `AfterFunc` goroutine that would take the
process down. The Close hook SHALL be called at most once per incarnation. Because
the table waits, a Close hook that blocks blocks whoever ended the incarnation; Close hooks
SHALL NOT block. Every goroutine the table starts for a key SHALL exit once that key's holder
contexts are Done and its incarnation has ended.

When the ending incarnation stored `Hooks.EnforceCloseBeforeOpen`, the table SHALL keep the key
stored until Close has returned, so a concurrent `Open` for that key waits for Close instead of
creating while it is in flight. After Close returns the key SHALL NOT be stored. When that field
is false (the zero value), the table SHALL unmap the key before Close, so a concurrent `Open` MAY
create while Close is still in flight. Close SHALL NOT run while the table mutex is held. The
Close window SHALL NOT be a sleeping window: an `Open` that arrives during Close MUST NOT wake
the ending incarnation.

Tests-only `Reset` MAY unmap first regardless of the field. Callers MUST NOT race `Reset` with
`Open` on the same key.

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

#### Scenario: Create does not start while previous Close is in flight
- **WHEN** Close is in flight for a key (zero grace, or after grace elapsed)
- **AND** that incarnation stored `Hooks.EnforceCloseBeforeOpen`
- **AND** `Open` is called for that key
- **THEN** `create` does not run until Close has returned
- **AND** that `Open` does not reclaim the closing value

#### Scenario: Create starts while previous Close is in flight by default
- **WHEN** Close is in flight for a key (zero grace)
- **AND** that incarnation did not store `Hooks.EnforceCloseBeforeOpen`
- **AND** `Open` is called for that key
- **THEN** `Open` returns a new incarnation while Close is still blocked
- **AND** that `Open` does not reclaim the closing value

### Requirement: Library Open loads under Traefik Yaegi
A Traefik local plugin SHALL import this module's `reclaim` package, hold one table created with
`New(Config)`, and call that table's `Open` from `New` with `Hooks` that log sleep, wake, and
close. Traefik SHALL start. A request through that plugin SHALL succeed. Two plugin instances that
Open the same key on that table SHALL receive the same stored value. Those hooks SHALL run under
Yaegi. Inert hooks MUST NOT be accepted as success for this load.

#### Scenario: Fake plugin starts and shares one incarnation
- **WHEN** Traefik v3.7.11 loads a local plugin whose `New` calls `Open` on a caller-owned table for a shared key
- **AND** two routes each construct that plugin
- **THEN** Traefik's API is reachable
- **AND** a request through each route succeeds
- **AND** both instances observe the same stored value identity
- **AND** Traefik logs include `reclaim_put` and `reclaim_bind`

#### Scenario: Reload runs sleep then wake hooks
- **WHEN** both plugin instances for that shared key are torn down and constructed again within grace
- **THEN** Traefik logs include `reclaim_orphan` and `reclaim_reclaim`
- **AND** Traefik logs include the plugin's sleep hook line and wake hook line

#### Scenario: Teardown runs the close hook
- **WHEN** every plugin instance for that shared key is torn down and grace elapses
- **THEN** Traefik logs include `reclaim_dispose`
- **AND** Traefik logs include the plugin's close hook line

### Requirement: Create panic or nil create is a create error
A nil `create` SHALL be rejected alongside the other `Open` argument guards, before the key is registered, so no slot is ever mapped for it. A `create` that panics SHALL be treated the same as `create` returning an error: the key SHALL NOT be stored, every caller waiting on that create SHALL receive an error, and a later `Open` SHALL be free to try again. The table SHALL NOT call Close in either case (nothing was stored). A panic SHALL be wrapped as `fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)`. A nil `create` SHALL be reported as `fmt.Errorf("reclaim: create %q: nil create", key)`.

#### Scenario: Create panic unsticks the key
- **WHEN** the first `Open` for a key has a `create` that panics
- **THEN** that `Open` returns an error wrapping the panic
- **AND** the key is not stored
- **AND** a later `Open` for that key with a valid `create` returns a value

#### Scenario: Nil create unsticks the key
- **WHEN** `Open` is called with a nil `create`
- **THEN** `Open` returns an error
- **AND** the key is not stored
- **AND** a later `Open` for that key with a valid `create` returns a value
