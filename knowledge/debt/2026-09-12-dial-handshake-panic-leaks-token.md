# Dial AUTH/SELECT panic still leaks the in-use-turn token

IssueKey: 2026-09-12-simpleredis-bug-01-panic-leaks-pool-token
Size: large
Action: note

## Why this follow-up
`borrow` holds the in-use-turn through `dial()`. AUTH and SELECT call `do` there with no deferred `release`. A panic in handshake `do` still drops the token and the socket.

## Why it was not taken
`requirement.md` Out of scope: wrapping `do` inside `dial` AUTH/SELECT. This run’s defer is on `exec` after `borrow` returns.

## Risks
`PoolSize` handshake panics still empty the semaphore. Later commands wait `PoolTimeout` and return `redis:unreachable` for the life of the client.

## Context
Current: `simpleredis/pool.go` `dial` AUTH/SELECT `do` while the caller of `borrow` still owns the token.
This change: `exec` only, after `borrow` returns a socket.
