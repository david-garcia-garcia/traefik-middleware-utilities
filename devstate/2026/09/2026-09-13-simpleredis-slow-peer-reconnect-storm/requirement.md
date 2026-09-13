# Requirement
IssueKey: 2026-09-13-simpleredis-slow-peer-reconnect-storm

## Problem
A Redis peer that answers just above `IOTimeout` makes `simpleredis` close the pooled socket on every command. Idle reuse drops to zero: connect-per-command. The slow peer then receives a dial storm (plus AUTH/SELECT when those knobs are set) on top of the load that made it slow. Closing the timed-out socket is correct; governing the *dial rate* may not belong in this package. The simplicity gate overrides landing a fix: if the smallest correct change is not small, coherent, and elegant, stop after propose with options and costs. "Do not fix in code" is a successful outcome.

## Current (code)
- `simpleredis/resp.go` `do` — `SetDeadline` to `min(IOTimeout, ctx remaining)`. Write or read error (including `os.ErrDeadlineExceeded`) returns `reusable false` via `ioOrContext`. The reply is still outstanding, so the socket cannot be parked.
- `simpleredis/commands_exec.go` `runOnConn` — `reusable` starts false; `defer release(conn, reusable)`; `do` may set it true only on a clean reply with empty reader. Timeout leaves it false.
- `simpleredis/pool.go` `release` — `!reusable` closes the TCP socket then `freeInUseTurn`. No park. Next `borrow` misses idle and `dial`s.
- `simpleredis/pool.go` `dial` — new TCP, then `do` AUTH when `Pass` is set and `do` SELECT when `Database` is set. Each handshake hop is another `IOTimeout` round trip on a fresh socket.
- `simpleredis/commands_exec.go` `shouldRetry` / `isCommandTimeout` — `redis:timeout` is never retried (spec deviation from go-redis). Retry backoff in `retryBackoff` therefore does not run on this path. Independent commands each dial once and fail.
- `simpleredis/config.go` — `defaultIOTimeout` is 100ms. `applyDefaults` maps `IOTimeout <= 0` to that. Comment on `Config`: worst-case wait `(MaxRetries+1)*(DialTimeout+IOTimeout)` (600ms at zeros).
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — `redis:timeout` MUST NOT be retried; "A timeout on a reused connection MUST NOT open a second connection" is the same-command retry rule, not a cap on later independent commands.
- `knowledge/devdocs/std_go_simpleredis.md` — documents 100ms `IOTimeout`, timeout not retried, AUTH/SELECT on each new dial. No operator note on timeout → connect-per-command vs `maxclients`.
- `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` — existing large note: fail-fast after k consecutive *dial* failures. Not taken (new failure-memory policy). Different trigger (dead peer vs slow-but-alive peer).
- Reproduction named by the ticket (`simpleredis/PRODUCTION-BUGS.md` BUG-5, `simpleredis/bugs_production_test.go` `TestBugSlowPeerDestroysEveryPooledSocket` behind `//go:build simpleredis_bugs`) — **not found** on `origin/master`. Present untracked in the original checkout. Dest default suite has no test that bounds dials under a peer slower than `IOTimeout`.
- `simpleredis/BUGS.md` on dest — does not list this reconnect storm.

## Desired
- Confirm the 15-command / 15-dial / 0-idle measurement independently. Quantify extra connections per second at a given request rate, and the AUTH+SELECT handshake tax on each reconnect.
- Decide whether governing dial rate belongs in this package. Closing a socket with an outstanding reply stays required.
- Weigh cheap non-breaker options: a better default `IOTimeout` (human decision; behaviour change for every existing caller), whether retry backoff can dampen this (dest code says no: timeouts are not retried), vs documenting `IOTimeout` × `PoolSize` vs server `maxclients`.
- Implement only if a genuinely small, coherent, Yaegi-safe change falls out. No background goroutine or shared mutable breaker/limiter state unless that still meets the simplicity gate. Preserve in-use-turn soundness, `OverFrees() == 0`, no fd or goroutine leaks. Surgical diff: only `simpleredis`, Go 1.21 stdlib, comment style matches dest.
- If implementing: a permanent untagged default-suite test that fails before and passes after, showing connection churn is bounded under a peer slower than `IOTimeout`. Prefix new fakes/helpers so they cannot collide with sibling branches.
- If not implementing: measurements, options with costs, recommendation, and a documentation change. No behaviour-changing product PR. Stub PR for bus/docs is OK.
- Do not change default `IOTimeout` in this run without an explicit human decision recorded as such.

## Affected
- `simpleredis/commands_exec.go` (`exec`, `runOnConn`, `shouldRetry`) — only if a code fix is chosen
- `simpleredis/pool.go` (`release`, `dial`) — only if a code fix is chosen
- `simpleredis/resp.go` `do` — only if a code fix is chosen
- `simpleredis/config.go` — only if the human decides a default `IOTimeout` change
- `knowledge/devdocs/std_go_simpleredis.md` and/or `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — docs path even when no code lands
- Default-suite test under `simpleredis/` — only if a code fix is chosen

## Out of scope
- Other packages (`tokenbucket`, `windowcounter`, Traefik middlewares, `backendbackoff`)
- Shipping a circuit breaker or dial-rate limiter "to close the ticket" when that is not small and elegant
- Changing default `IOTimeout` as an agent decision
- Background reaper / idle-sweep work (BUG-6 / existing idle-reaper debt)
- The existing dial-failure breaker debt as this ticket's implementation (different trigger; already noted large)
- Untracked `PRODUCTION-BUGS.md` / `bugs_production_test.go` as files to merge unless a later phase chooses a dest-owned test
- Sibling `simpleredis` bugs in parallel (same files likely)
- Implementing or proposing in prepare

## Unknowns
- Independent confirmation of 15 dials / 0 idle (prepare did not run the tagged reproduction).
- Extra connections per second at production request rates, and AUTH+SELECT cost, not yet measured.
- Whether any non-breaker change is small and coherent enough to implement.
- Whether this package is the right owner of dial-rate policy vs operator `IOTimeout` / `PoolSize` / Redis `maxclients`.
- Merge collisions with in-flight branches that also edit `commands_exec.go`, `pool.go`, `resp.go`.

## Tensions
- Socket close on timeout is specified and correct; the ticket's defect is the *ungoverned dial rate across commands*, not the close.
- Spec "timeout on a reused connection MUST NOT open a second connection" forbids retry-on-timeout of the same command. Dest already obeys that. The storm is the next independent command dialing after a correct close.
- Ticket asks to consider retry backoff as a damper. Dest never retries `redis:timeout`, so backoff does not run. Documenting that is part of the answer, not a code surprise.
- Default `IOTimeout` 100ms may be the operator lever; changing it is a behaviour change for every caller and is a human decision, not this run's.
- Textbook breaker/limiter vs package identity (stdlib-only, Yaegi-safe, no extra config surface). Simplicity gate says concluding "document, do not ship a breaker" is success. Existing `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` already refused a similar policy for dial *failures*.
- Five sibling agents editing `simpleredis` in parallel; any later diff must stay surgical.
