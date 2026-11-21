## ADDED Requirements

### Requirement: Bind-time finished snapshot is synchronized
When `Open` starts a watcher for a holder whose `Done` is nil, the table SHALL read that
incarnation's end-signal channel while holding the table mutex. If that snapshot is nil, the
incarnation has already ended: the table SHALL NOT start a watcher. A watcher SHALL NOT be
started with a nil end-signal channel. When the snapshot is non-nil, the watcher SHALL still
exit without dropping that holder if the incarnation ends first, even if `ctx.Err()` stays unset.

#### Scenario: Nil-Done bind during Reset does not leak a watcher
- **WHEN** a live incarnation has a keep-alive holder
- **AND** `Open` is called with a holder whose `Done` is nil and whose first `Err` is delayed
- **AND** `Reset` ends that incarnation while that `Open` is parked in `Err`
- **THEN** after every incarnation has ended the table owns no more goroutines than it did before those `Open` calls
- **AND** that `Open` does not leave a watcher polling for the life of the process

### Requirement: Table mutex is released if a panic unwinds a lock-held region
Every region that holds the table mutex SHALL release it with `defer`, so a panic raised while
the mutex is held still unlocks during unwinding. After such a panic is recovered (as the Yaegi
plugin boundary would), a later `Open` or `Reset` on that same table SHALL be able to acquire the
mutex. Close, Sleep, Wake, create, and every log line SHALL still run outside the mutex. Channel
closes that the table already performs outside the mutex SHALL stay outside it.

#### Scenario: Panic under the table mutex does not wedge later callers
- **WHEN** a panic is raised while the table mutex is held
- **AND** that panic is recovered
- **THEN** a later `Open` or `Reset` on that table completes instead of blocking forever on the mutex

### Requirement: Uninitialized table is rejected
`Open` SHALL return an error when the table has a nil items map (a zero-value `Table{}`), the
same class of rejection as a nil table pointer or a nil logger. `Open` MUST NOT panic on that
path. `New(Config)` remains the constructor that allocates the map.

#### Scenario: Zero-value table Open returns an error
- **WHEN** `Open` is called on a zero-value `Table{}`
- **THEN** `Open` returns an error
- **AND** `Open` does not panic
