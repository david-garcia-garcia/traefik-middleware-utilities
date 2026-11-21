## MODIFIED Requirements

### Requirement: Grace is configurable
Grace SHALL be how long a **sleeping** value is kept before it is disposed. Because a sleeping
value has released what is expensive to hold idle, a long grace is cheap: the reason to keep it
long is that a sleeping value costs little, not that reloads are fast. The table SHALL use a
caller-supplied grace duration. A zero grace SHALL dispose of the value as soon as the last
holder is gone, with no sleeping window at all. A negative grace SHALL become the product default
of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`) when the
process table is constructed.

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
