# Close sockets abandoned by a panic between borrow and release

IssueKey: 2026-09-13-simpleredis-lost-turn-recovery
Size: large
Action: note

## Why this follow-up
A panic between `borrow` and `release` leaks that socket's fd permanently. Lost-turn recovery dials a new socket rather than reclaiming the abandoned one, so the client recovers concurrency but accumulates one dead fd per panic, unbounded.

## Why it was not taken
This change fixed turn permanence only. Closing the fd needs tracking of checked-out sockets so recovery can close the abandoned one, which is a different owner than refill-at-pool-wait.

## Risks
A plugin that panics once an hour leaks about 24 fds a day. Fd exhaustion is the same class of outage as the turn brick, just slower.

## Context
Current: `simpleredis/commands_exec.go` `exec` does not defer `release`; `simpleredis/pool.go` `recoverLostTurnsLocked` refills turns and the next command dials.
Proposed: track checked-out sockets so recovery can close the abandoned fd instead of only minting a new turn.
