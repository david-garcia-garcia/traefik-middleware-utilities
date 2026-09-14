## MODIFIED Requirements

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while sleeping, `Reset`, a zero-grace drop, a Sleep
panic, or a Wake panic), the table SHALL call the Close hook when that func is non-nil and
SHALL wait until it has returned, or until its panic has been recovered and reported, before it
emits `reclaim_dispose`. The table SHALL NOT let a Close panic escape, because Close can run on an
`AfterFunc` goroutine that would take the process down. The Close hook SHALL be called at most
once per incarnation. Because the table waits, a Close hook that blocks blocks whoever ended the
incarnation; Close hooks SHALL NOT block. Every goroutine the table starts for a key SHALL exit
once that key's holder contexts are Done and its incarnation has ended.

When the ending incarnation stored `Hooks.EnforceCloseBeforeOpen`, the table SHALL keep the key
stored until Close has returned, so a concurrent `Open` for that key waits for Close instead of
creating while it is in flight. After Close returns the key SHALL NOT be stored. That keep-mapped
rule SHALL hold on a Sleep-panic ending regardless of grace, and on a Wake-panic ending. When
that field is false (the zero value), the table SHALL unmap the key before Close, so a concurrent
`Open` MAY create while Close is still in flight. Close SHALL NOT run while the table mutex is
held. The Close window SHALL NOT be a sleeping window: an `Open` that arrives during Close MUST
NOT wake the ending incarnation.

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
- **WHEN** Close is in flight for a key (zero grace, after grace elapsed, Sleep panic, or Wake panic)
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
