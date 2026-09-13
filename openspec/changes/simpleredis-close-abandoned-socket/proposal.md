## Why

Lost-turn recovery on DestBranch (PR #66) restores concurrency after a panic between `borrow` and `release`, then dials a **new** socket. The abandoned socket's fd stays open for the process lifetime. A plugin that panics once an hour leaks about 24 fds a day; fd exhaustion is the same outage class as the turn brick, slower.

## What Changes

- Keep each checked-out `*pooledConn` on a client registry with the checkout timestamp. Register when `borrow` hands the socket out; deregister in `release` (reuse and destroy).
- On the existing pool-wait recovery path, close registry entries whose checkout is older than the command budget already used by `bindCommandDeadline`: after `retryLimits`, `(maxRetries+1)*(DialTimeout+IOTimeout)`. Do not add a sweeper goroutine.
- Do not close a socket that is still within that budget (a live command, or a conn in the borrow-to-do / do-to-release gap).
- Export read-only `AbandonedClosed()` next to `LostTurns()`. `LostTurns()` still reports restored tokens.
- Default-suite tests: abandoned sockets eventually close (server-side open count bounded); a live in-flight command is never reclaimed (`-count=5`); a gap conn is not reclaimed; counters stay honest.
- Do not `defer sr.release`. Do not replace `heldSockets` for turn refill (held-socket-lease debt stays). Do not edit `simpleredis/BUGS.md`. This change stacks on PR #66; #66 must merge first.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: recovered panics MUST eventually close the abandoned fd. Reclaim only from pool-wait, and only when checkout age is at least the command budget. A live command and a gap conn MUST NOT be closed. `AbandonedClosed()` SHALL be readable next to `LostTurns()`.

## Impact

- `simpleredis/pool.go` (checkout registry, reclaim on pool-wait).
- `simpleredis/simpleredis.go` (`AbandonedClosed()`; registry field).
- `simpleredis/commands_exec.go` only if the budget helper is shared with `bindCommandDeadline`.
- `simpleredis/pool_test.go` (or a sibling untagged `_test.go`).
- Usage packet `knowledge/devdocs/std_go_simpleredis.md`.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Take dest debt `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md` (delete the file). Leave `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`.
