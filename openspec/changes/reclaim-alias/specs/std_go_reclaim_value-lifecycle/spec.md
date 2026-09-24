## ADDED Requirements

### Requirement: SetAlias publishes a mapped key under a public alias
The table SHALL allow a publisher to bind an awake or asleep mapped key to an opaque alias name
without moving holder counts. Holders SHALL remain on the ownership key. A second publisher on the
same alias SHALL be rejected while the first publisher still holds it. The same publisher in the
same group MAY replace its own alias (rename): the table SHALL clear watchers on the old alias
name before publishing the new one.

#### Scenario: Publish updates existing watchers
- **WHEN** watchers are registered for an alias before any publish
- **AND** `SetAlias` maps a key to that alias
- **THEN** those watchers receive the mapped value

#### Scenario: Conflicting publisher is rejected
- **WHEN** alias `A` is held by publisher `P1`
- **AND** publisher `P2` attempts `SetAlias` on `A`
- **THEN** the call returns an error
- **AND** `P1` keeps the alias

#### Scenario: Same-publisher rename clears old alias watchers
- **WHEN** publisher `P` in group `G` holds alias `old`
- **AND** `P` publishes the same key under alias `new` in group `G`
- **THEN** watchers on `old` see empty
- **AND** watchers on `new` see the mapped value

#### Scenario: SetAlias requires a mapped awake or asleep key
- **WHEN** `SetAlias` is called for a missing key or a slot that is not awake or asleep
- **THEN** the call returns an error

### Requirement: Watch is a weak subscription on an alias
`Watch` SHALL register a callback on an alias without binding a holder, without stopping grace, and
without waking a sleeping value. It SHALL deliver `Published{Value}` when the published value
changes. The first non-nil empty value supplied to `Watch` SHALL stick for that alias entry. The
same pointer value SHALL NOT invoke the callback twice. A nil callback SHALL skip delivery. When
the subscriber context is done, the table SHALL drop that subscriber without closing the mapped
value. `Watch` with a nil context SHALL panic.

#### Scenario: Watch before publish leaves typed empty
- **WHEN** `Watch` runs before any `SetAlias` for that alias
- **THEN** the subscriber sees the empty value
- **AND** no holder is bound

#### Scenario: Changed fires once per distinct value
- **WHEN** an alias publishes value `V`
- **AND** the same alias publishes `V` again
- **THEN** the callback runs once for the first publish
- **AND** does not run again for the identical pointer republish

#### Scenario: ClearPublisher clears watchers to empty
- **WHEN** `ClearPublisher` runs for a publisher and group that holds aliases
- **THEN** watchers on those aliases see empty
- **AND** watchers remain registered for a later publish

#### Scenario: Context cancel drops subscriber only
- **WHEN** a watcher context ends before a later publish
- **THEN** that subscriber receives no further deliveries
- **AND** the mapped incarnation is unchanged

### Requirement: ClearPublisher is scoped by publisher and group
`ClearPublisher` SHALL clear only aliases whose stored publisher and group match the arguments.
It SHALL NOT clear aliases held by another group for the same publisher name.

#### Scenario: Independent groups do not cross-clear
- **WHEN** publisher `P` holds alias `A` in group `G1` and alias `B` in group `G2`
- **AND** `ClearPublisher(P, G1)` runs
- **THEN** watchers on `A` see empty
- **AND** watchers on `B` still see the published value

### Requirement: Incarnation end clears alias bindings without clobbering replacements
When an incarnation ends (unmap, failed create path, enforced close, grace dispose after close,
or table reset that takes all slots), the table SHALL detach every alias entry pointing at that
incarnation, clear publisher metadata on those entries, and deliver empty to their watchers. If a
replacement publisher already bound the same alias to a new incarnation, teardown of the old
incarnation SHALL NOT overwrite that replacement's published value.

#### Scenario: Unmap clears alias watchers
- **WHEN** a key with a published alias is unmapped
- **THEN** watchers on that alias see empty
- **AND** a later `SetAlias` may publish again

#### Scenario: Grace dispose clears alias watchers
- **WHEN** a key orphans, grace elapses, and the incarnation is disposed
- **THEN** watchers on aliases that pointed at that incarnation see empty

#### Scenario: Table reset clears all aliases
- **WHEN** the table reset path takes all slots
- **THEN** every alias entry is cleared
- **AND** watchers see empty

#### Scenario: Dying incarnation does not clobber replacement publisher
- **WHEN** alias `A` is republished to a new incarnation while the old incarnation is still closing
- **THEN** watchers on `A` keep the replacement value
- **AND** teardown of the old incarnation does not force empty over the replacement

### Requirement: Peek reads without binding or waiting
`Peek` SHALL return the stored value and whether the slot is awake or asleep when the key exists
and the slot is not busy. It SHALL return `ok=false` for a missing key, a gone slot, or a busy
slot (create, wake, sleep, or close in flight). `Peek` SHALL NOT wait on a busy slot, SHALL NOT
bind a holder, SHALL NOT wake a sleeping value, and SHALL NOT shorten grace or run close early.

#### Scenario: Missing key returns not ok
- **WHEN** `Peek` is called for a key that was never mapped
- **THEN** `ok` is false

#### Scenario: Busy slot returns not ok without blocking
- **WHEN** a slot is busy with create in flight
- **AND** `Peek` is called concurrently
- **THEN** `Peek` returns promptly with `ok` false

#### Scenario: Awake peek does not bind
- **WHEN** a key has a single holder and `Peek` returns awake with `ok` true
- **AND** that holder's context is canceled
- **THEN** the slot may sleep without a peek having counted as a holder

#### Scenario: Asleep peek does not accelerate grace
- **WHEN** a key is asleep within grace
- **AND** `Peek` returns asleep with `ok` true
- **THEN** grace is not shortened
- **AND** after grace elapses without reclaim, `Peek` returns `ok` false
