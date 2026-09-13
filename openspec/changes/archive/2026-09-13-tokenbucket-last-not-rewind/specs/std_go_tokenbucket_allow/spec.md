## ADDED Requirements

### Requirement: Persisted last does not rewind
The in-process store SHALL persist `last` as the later of the previous stored `last` and this consume's now (Unix microseconds). Elapsed for refill SHALL still treat a now behind `last` as zero elapsed so refill is never negative. The store MUST NOT persist a `last` earlier than the previous stored `last`. The in-process store SHALL read now after it holds the mutex that guards the bucket map, so lock order is clock order. Tests SHALL fail on dest for a sequential backward now and for a stale now sampled before a newer consume finishes, then pass after this change.

#### Scenario: Sequential backward now does not refill twice
- **WHEN** a key is admitted at t0 then at t2 (burst 1, maxDelay 0, rate 1)
- **AND** Allow is called with now at t1 (between t0 and t2)
- **THEN** that Allow is denied
- **AND** the stored `last` stays at t2
- **WHEN** Allow is called again with now at t2
- **THEN** that Allow is denied
- **AND** MUST NOT admit by refilling the t2-minus-t1 interval already granted

#### Scenario: Stale now sampled before lock does not rewind last
- **WHEN** two Allows overlap on one key: the first samples t1 and waits outside the mutex, the second samples t2 and runs to completion first
- **THEN** the first MUST NOT store `last` as t1 after the second stored t2
- **WHEN** Allow is called later with now at t2
- **THEN** that Allow is denied
