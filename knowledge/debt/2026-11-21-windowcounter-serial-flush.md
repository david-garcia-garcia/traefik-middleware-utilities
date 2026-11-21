# Bound the duration of a buffered `windowcounter` flush with many dirty keys

IssueKey: 2026-11-21-ratelimit-benchmarks
Size: large
Action: note

## Why this follow-up
`flushPendingLocked` sends one EVAL per dirty key, one after another. Probe on 2026-11-21 against the in-process fake: about 21 µs per key, so 10,000 dirty keys took 211 ms. Against a networked Redis the duration is N × RTT. Once that is longer than `sync_rate`, the real sync interval grows with the number of keys.

## Why it was not taken
The ticket only adds benchmarks and README text. `requirement.md` puts optimising the limiters out of scope. Pipelining or batching the flush changes the `std_go_windowcounter_sync-flush` contract.

## Risks
Instances with many active keys share counts less often than `sync_rate` promises, so the combined admit count across instances can exceed `limit` by more than the README implies.

## Context
Current: `windowcounter/limiter.go` `flushPendingLocked` / `flushEvalUnlocked`.
Proposed: pipeline the per-key EVALs, or send one script per batch of keys (keys listed in `KEYS` for Dragonfly).
