# Requirement
IssueKey: 2026-09-13-simpleredis-stale-pooled-socket-retry

## Problem
After every idle pooled socket is dropped server-side (Redis restart, failover, `CLIENT KILL`, or a server idle-timeout reap), sequential commands fail with `redis:unreachable` while the peer stays healthy and accepting. The burst is `PoolSize / (MaxRetries+1)` consecutive failures (defaults: PoolSize 8, MaxRetries 1 → 4). Parallel callers hide it by draining corpses together; the quiet path does not.

## Current (code)
- `simpleredis/pool.go` `borrow` — after `takeIdleConn`, a non-nil `reused` is returned with `handedOff = true` and `handshakeFailed false`. An idle miss dials. The three-value return does not tell `exec` whether the socket came off idle or was freshly dialled. `handedOff` only gates `freeInUseTurn` on panic/error paths; it is not a reuse signal.
- `simpleredis/pool.go` `takeIdleConn` — LIFO pop of a still-young idle socket. No liveness probe. No generation/epoch. Server-closed sockets younger than `IdleTimeout` stay on the list until borrowed.
- `simpleredis/commands_exec.go` `exec` — `for attempt := 0; attempt <= maxRetries`; `borrow` then `runOnConn`. `errUnreachable` from `do` is retryable via `shouldRetry(err, false)`. The next `borrow` can pop the next idle corpse. Each command spends at most `MaxRetries+1` corpses, then returns `redis:unreachable`.
- `simpleredis/commands_exec.go` `runOnConn` / `shouldRetry` / `isUnreachable` — I/O on a reused socket and a down peer both become the unreachable sentinel. Identity compare so `errPoolWait` / `errNotFromNew` are not retried.
- `simpleredis/resp.go` `do` / `ioError` — write/read failure after `SetDeadline` maps EOF to `errUnreachable` (`reusable=false`). `release` then closes that socket.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — "Peer-closed idle socket is retried": one idle socket, next command retries on a new dial under `MaxRetries`. Live CLIENT KILL scenarios also assume one pooled connection.
- `simpleredis/pool_e2e_test.go` `runLivePeerCloseRecovery` — asserts `idle == 1`, `CLIENT KILL` that ADDR, next Get succeeds. Does not fill the idle list.
- `simpleredis/pool_test.go` — sequential reuse, idle-timeout sweep, `assertTurnsFullAndNoOverFrees`. No test that drops every accepted socket then issues sequential Gets.
- `simpleredis/fake_redis_test.go` `fakeRedis` — counts accepts; does not keep a slice of accepted sockets to drop them all. `holdGetsForTest` can warm simultaneous in-flight Gets.
- `knowledge/devdocs/std_go_simpleredis.md` — "A server-closed idle socket younger than 30s is borrowed and retried as redis:unreachable". Worded for one socket.
- `simpleredis/PRODUCTION-BUGS.md` BUG-1 and `simpleredis/bugs_production_test.go` `TestBugDeadIdle*` — not found on dest (untracked in the caller checkout, build tag `simpleredis_bugs`).
- Pool generation / epoch field on `SimpleRedis` — not found.

## Desired
- Sequential commands after every idle socket is dropped succeed while the peer is accepting. No config knob. Hard-bounded so a reused-socket I/O failure cannot loop.
- `borrow` reports whether the handed socket was reused from idle. `exec` treats I/O failure on a reused socket as not evidence about the peer: that failure does not consume the retry budget, and the next attempt is forced onto a fresh dial (must not pop the next idle corpse).
- Permanent default-suite test (no build tag): real TCP, in-process RESP fake, warm idle with genuinely simultaneous in-flight commands, drop every server socket, sequential requests succeed. New fake/helper types use a bug-specific prefix. Reuse `simpleredis/fake_redis_test.go` helpers where they fit.
- In-use-turn semaphore stays sound (`OverFrees() == 0`), no fd leak, no goroutine leak. Diff surgical: only `simpleredis`; BUG-6 sibling also touches `pool.go` borrow paths.
- Package constraints unchanged: Go 1.21, stdlib already used, Yaegi-interpretable, no generics, no reflection. Comment style: constraint or reason, never restate the code.
- Simplicity gate: if the cheapest correct fix is not small, coherent, and elegant, stop after propose with written options. A pool-generation / epoch mechanism is the textbook shape and a strong signal to stop and ask rather than ship it.

## Affected
- `simpleredis/pool.go` — `borrow` reuse signal and any force-dial path
- `simpleredis/commands_exec.go` — `exec` retry accounting on reused-socket I/O
- new untagged `simpleredis/*_test.go` (bug-specific type prefix)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — peer-closed idle coverage today is one socket
- `knowledge/devdocs/std_go_simpleredis.md` — one-socket retry wording

## Out of scope
- PRODUCTION-BUGS BUG-2 through BUG-6
- Other packages
- A new Config knob
- Shipping a pool-generation / epoch mechanism (evaluate; if that is what it takes, stop after propose)
- Branch `2026-09-13-simpleredis-clamp-maxidleconns-to-poolsize` and other sibling IssueKeys
- Committing the tagged `bugs_production_test.go` / `PRODUCTION-BUGS.md` as the permanent test
- Changing AUTH/SELECT handshake no-retry, pool-wait no-retry, or timeout no-retry
- Writable pool/timeout/retry fields; new imports; `unsafe` / cgo / generics

## Unknowns
- Whether `MaxRetries: -1` (one send today) still gets one hard-bounded reused-socket redial, or only commands that already have a retry slot recover.
- Whether leftover idle corpses after a successful force-dial stay on the list for later commands (each later command spends one corpse then dials) or must be discarded at the first reused EOF.
- How "force a fresh dial" is plumbed into `borrow` without a config knob and without colliding with a sibling idle-reaper change on `takeIdleConn`.
- Whether a fourth `borrow` return collides with in-flight sibling PRs (existing nolint: error stays before `handshakeFailed`).

## Tensions
- Dest spec and live CLIENT KILL tests require recovery after **one** peer-closed idle socket. They pass on dest. The measured defect is **N** idle sockets: each command burns `MaxRetries+1` corpses and the rest of the vintage stays. Desired is the N-socket sequential path, not a rewrite of the one-socket proof.
- Ticket cheap shape (reuse flag + skip idle on the next attempt, hard bound) vs textbook epoch. Epoch is named as likely over-engineered; if the cheap shape cannot be made correct without it, stop at the gate.
- "Does not consume the retry budget" vs spec retry loop `attempt := 0; attempt <= maxRetries`. A free extra send when MaxRetries is off is a reshape of that bound. Explore records the choice; do not silently expand retry.
- `handedOff` in `borrow` already means "turn transferred to the caller", not "socket came from idle". Do not overload it as the reuse signal.
- Simplicity gate overrides workflow `Done when`: stopping after propose with costs written is success for this ticket.
