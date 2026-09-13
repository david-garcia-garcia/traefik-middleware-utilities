# Lease checked-out sockets so recovery cannot refill while a socket is live

IssueKey: 2026-09-13-simpleredis-lost-turn-recovery
Size: large
Action: note

## Why this follow-up
`heldSockets` covers a socket only while a command is running on it. Between `borrow` and `doWithHeldSocket`, and between that defer and `release`, idle is empty and `heldSockets` is 0 while a socket is still live. If every `PoolSize` holder sits in one of those gaps when a waiter's `PoolTimeout` fires, recovery refills and may dial past the cap.

## Why it was not taken
Counting the socket in `borrow` and decrementing in `release` would leave a panic-leaked socket at `heldSockets > 0` forever, so recovery would never fire. Closing the window needs a lease token or an activity timestamp per checked-out socket, which is a different design.

## Risks
A simultaneous pool-wait expiry can refill spuriously. The overshoot is bounded by `PoolSize` and self-corrects via `OverFrees` when holders release. Treating that as the cap would leave a reader believing `PoolSize` is unconditional.

## Context
Current: `simpleredis/pool.go` `recoverLostTurnsLocked`; `simpleredis/commands_exec.go` `doWithHeldSocket`.
Proposed: a lease token or activity timestamp per checked-out socket so a live socket in the borrow-to-do or do-to-release gap still counts as owned, without pinning a panic-leaked fd as busy forever.
