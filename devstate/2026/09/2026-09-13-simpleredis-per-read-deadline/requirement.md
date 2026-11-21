# Requirement
IssueKey: 2026-09-13-simpleredis-per-read-deadline

## Problem
A bulk value that takes longer than `IOTimeout` to cross the wire is permanently unreadable. A compliant peer that streams steadily still fails every Get with `redis:timeout`, allocates the whole payload, and burns a fresh dial. Measured: 4 MiB with `IOTimeout` 60 ms failed 5/5, 20.4 MiB allocated, 5 dials. Raising `IOTimeout` is not the fix: that same bound is what times out every small command.

## Current (code)
- `do` applies one absolute `SetDeadline(now+ioBound)` for the whole command, with no extension on progress: `simpleredis/resp.go:22-31`.
- `ioBound` is `clampTimeout(ctx, sr.IOTimeout())`: remaining caller/library context time if shorter than `IOTimeout`: `simpleredis/commands_exec.go:107-121`; call at `simpleredis/resp.go:22`.
- Default `IOTimeout` is 100 ms (`0` at New maps to that): `simpleredis/config.go:10`, `:37-38`, `:65-66`; `IOTimeout()`: `simpleredis/simpleredis.go:164-169`.
- Decoder bulk ceiling is `maxBulkLength = 64 << 20`; over-cap is `errIssue` before `make`: `simpleredis/resp.go:205-208`, `:237`. Decode spec keeps `make([]byte, length+2)` + `ReadFull` for accepted lengths: `openspec/specs/std_go_simpleredis_resp-decode/spec.md:26`, `:57`.
- `exec` binds `(MaxRetries+1)*(DialTimeout+IOTimeout)` onto ctx when sooner than the parent; library expiry is `redis:timeout`, a sooner caller deadline stays `ctx.Err()`: `simpleredis/commands_exec.go:16-25`, `:74-91`.
- `watchConnClose` closes the socket when that ctx fires (`context.AfterFunc`; Yaegi-safe, no `select` on `ctx.Done`): `simpleredis/resp.go:32-34`, `:64-81`.
- `ioOrContext` maps a socket deadline to `context.DeadlineExceeded` when `ioBound < ioTimeout` (clamped caller instant), else `ioError` → `redis:timeout` via `errors.Is(os.ErrDeadlineExceeded)` (no `net.Error` assert): `simpleredis/resp.go:84-99`, `:286-292`.
- Caller vs library distinction is compiled: `simpleredis/commands_deadline_test.go` `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout` (`:193-201`) and `TestGetCallerDeadlineWhileWaitingForTurn`. Silent peer is `TestIoTimeout` (`simpleredis/resp_test.go:57-75`) and overall budget `TestBlackHoleGetReturnsWithinOverallDeadline` / `TestHandshakeStallIsBoundedByOverallDeadline` (`commands_deadline_test.go:35-69`).
- Tcp-session: per-command socket I/O SHALL still `SetDeadline` to the lesser of `IOTimeout` and time remaining; overall library expiry is `redis:timeout`: `openspec/specs/std_go_simpleredis_tcp-session/spec.md:217`, `:457`.
- Redis does not re-apply `proto-max-bulk-len` on GET replies; a client that trusts `$<len>` is unprotected by that server config: `knowledge/research/ext_redis_proto_max-bulk-len/notes.md`. go-redis `ReadTimeout` default is 5s on the pinned Options; no research folder states whether go-redis refreshes the deadline per `Read`: `knowledge/research/ext_go-redis_connection-pool/notes.md`.
- Tagged repro (untracked on dest; caller checkout): `simpleredis/PRODUCTION-BUGS.md` BUG-3; `simpleredis/bugs_production_test.go` `TestBugValueLargerThanIOTimeoutIsPermanentlyUnfetchable` + `startTrickleBulkRedis`. Dest default suite has no streaming-beyond-`IOTimeout` success test.

## Desired
- Treat `IOTimeout` as a stall bound, not a total-transfer timeout: refresh the read deadline when bytes actually arrive (smallest shape likely a wrapper that `SetReadDeadline(now+IOTimeout)` before each `Read`).
- Keep the overall command budget authoritative: a slow-drip peer must not pin an in-use turn past `(MaxRetries+1)*(DialTimeout+IOTimeout)` (`watchConnClose` / `bindCommandDeadline`).
- Keep `clampTimeout` / caller deadline authoritative. Caller deadline stays `context.DeadlineExceeded`; library budget stays `redis:timeout`. Do not break `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout`.
- Reconcile `maxBulkLength` with what the configured `IOTimeout` (and overall budget) can actually carry so the decoder does not advertise sizes it cannot deliver. Do not change how an accepted payload is allocated (`make` + `ReadFull`) — that is BUG-4's surface.
- Permanent untagged tests (bug-specific helper names so siblings do not collide): a compliant fake streaming a multi-megabyte bulk in chunks over a period well beyond `IOTimeout` returns the value intact; a peer that goes silent mid-reply still times out promptly.
- Only `simpleredis`. Go 1.21, stdlib, Yaegi-interpretable: no reflection, no `syscall`, no new non-stdlib imports; keep `errors.Is` for deadlines.
- Simplicity gate (later phases): if the simplest correct fix is not small, coherent, and elegant, do not implement it. Prepare still grounds the ticket.

## Affected
- `simpleredis/resp.go` (`do` deadline, possibly a stall wrapper; `maxBulkLength` only if reconciliation changes the const or its comment)
- `simpleredis/config.go` (`IOTimeout` comment: per-command `SetDeadline` vs stall)
- `simpleredis/commands_exec.go` only if `clampTimeout` / `ioOrContext` contract needs a new hook; prefer not
- `simpleredis/*_test.go` (new untagged stall vs stream tests; keep `commands_deadline_test.go`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (single `SetDeadline` wording vs stall refresh; overall budget unchanged)
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` only if `maxBulkLength` or “sizes we will deliver” changes

## Out of scope
- Any package other than `simpleredis`
- BUG-4 allocation shape (`readBulk` `make` before payload; incremental grow)
- BUG-5 timeout-driven connection storm / breaker
- BUG-1 / BUG-2 / BUG-6
- Raising default `IOTimeout` as the fix
- Tagged `simpleredis_bugs` files as the permanent suite (`PRODUCTION-BUGS.md`, `bugs_production_test.go` stay caller-untracked)
- Changing handshake AUTH/SELECT beyond whatever `do` deadline change they inherit
- `net.Error` asserts, Yaegi `select` on `ctx.Done`, non-stdlib imports

## Unknowns
- Whether go-redis `ReadTimeout` is a per-`Read` stall or a total-command deadline is not in `knowledge/research/` (only the 5s default).
- How to reconcile `maxBulkLength` with `IOTimeout` without shrinking the landed 64 MiB allocation ceiling: remaining overall budget, a derived cap from `IOTimeout`, or leave 64 MiB if stall-timeout plus `watchConnClose` is enough.
- Whether writes also need stall refresh; the ticket names read.
- Whether `bufio.Reader` plus a per-`Read` wrapper is enough, or `SetDeadline` on the raw conn races with buffered leftover.
- Whether a slow-but-compliant 4 MiB GET can finish inside the default overall budget `(1+1)*(200ms+100ms)=600ms` even after stall refresh; if not, the success test must set `MaxRetries` / timeouts so the overall cap is not the failure mode.

## Tensions
- Tcp-session requires one per-command `SetDeadline(min(IOTimeout, remaining))`. Stall-timeout is a different contract (refresh on progress). Spec must change if the fix lands; the overall budget line stays.
- Ticket “reconcile `maxBulkLength`” vs “do not also change how the payload is allocated” (BUG-4 sibling also edits `readBulk`) vs landed decode spec `64 << 20`. Shrinking the const is a spec revert; leaving it is advertising sizes the default overall budget still cannot finish.
- `ioOrContext` uses `ioBound < ioTimeout` to tell caller clamp from library I/O. Per-read refresh that always uses full `IOTimeout` (or remaining ctx) can collapse that distinction if `ioBound` is no longer the one-shot duration.
- Overall budget still multiplies `IOTimeout` as if it were a per-attempt total. A streaming value can hit library `redis:timeout` via `watchConnClose` even when the peer never stalls.
- Caller run name `TestBugValueLargerThanIOTimeout` is a `-run` prefix; dest/untracked name is `TestBugValueLargerThanIOTimeoutIsPermanentlyUnfetchable`.
- Simplicity gate overrides later Done-when; prepare still completes. Unattended full run is subject to that gate.
- Five sibling agents; keep the deadline diff off `readBulk` allocation.
