## MODIFIED Requirements

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
`reclaim_reclaim`.

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

#### Scenario: Recovered hook panic is reported
- **WHEN** a Sleep or Close hook panics and the table recovers it
- **THEN** logs include `reclaim_hook_panic` at error level for that key
- **AND** that line carries which hook panicked and the panic value

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while sleeping, `Reset`, or a zero-grace drop), the table
SHALL call the Close hook when that func is non-nil and SHALL wait until it has returned, or until
its panic has been recovered and reported, before it emits `reclaim_dispose`. The table SHALL NOT
let a Close panic escape, because Close can run on an `AfterFunc` goroutine that would take the
process down. The Close hook SHALL be called at most once per incarnation. Because
the table waits, a Close hook that blocks blocks whoever ended the incarnation; Close hooks
SHALL NOT block. Every goroutine the table starts for a key SHALL exit once that key's holder
contexts are Done and its incarnation has ended.

#### Scenario: Dispose log implies Close has returned
- **WHEN** a key is orphaned and grace elapses
- **AND** the Close hook is set
- **THEN** that func has returned before `reclaim_dispose` is emitted for that key
- **AND** that func did not observe a dispose line already written for that key

## ADDED Requirements

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
