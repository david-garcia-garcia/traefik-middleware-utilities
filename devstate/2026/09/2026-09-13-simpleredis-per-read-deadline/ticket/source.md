# Fix ONE production defect in simpleredis (BUG-3): IOTimeout as a total-transfer deadline

Fix ONE production defect in the `simpleredis` package.

A value that simply takes longer than `IOTimeout` to cross the wire can never be read. Not a flake, not a slow peer — the peer streams steadily and is perfectly compliant.

Measured: a 4 MiB value with `IOTimeout` at 60 ms failed 5 out of 5 attempts with `redis:timeout`, allocated 20.4 MiB, and burned 5 fresh TCP dials for nothing. The key is permanently unreadable.

Root cause: `IOTimeout` is applied once, as a single absolute deadline for the whole command, with no extension on progress (`simpleredis/resp.go` around line 29, inside `do`):

```go
	if err := conn.netConn.SetDeadline(time.Now().Add(ioBound)); err != nil {
		return nil, false, errUnreachable
	}
```

The default `IOTimeout` is 100 ms. The decoder simultaneously advertises support for payloads up to `maxBulkLength = 64 << 20` (`resp.go`, around line 237). Those two constants contradict each other: 64 MiB in 100 ms needs about 5.4 Gbit/s sustained. Everything above roughly a megabyte is unreachable at defaults, and the failure is indistinguishable from a genuine timeout.

Raising `IOTimeout` is not a fix, because the same timeout is what bounds every small command.

Reproduction (untracked in the main checkout; read by absolute path from the caller checkout if the worktree does not have them):
- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-3
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`
Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugValueLargerThanIOTimeout' -v`

Fix direction (evaluate, do not follow blindly): Make `IOTimeout` a **stall** timeout rather than a total-transfer timeout: refresh the read deadline whenever bytes actually arrive, so the bound is on the peer going quiet, not on how big the value is. The smallest shape is probably a tiny wrapper around the connection or the reader that calls `SetReadDeadline(now + IOTimeout)` before each `Read`.

Two things you must get right:
- The overall command budget must still bound the call. `exec` binds a context deadline of `(MaxRetries+1)*(DialTimeout+IOTimeout)`, and `watchConnClose` in `resp.go` closes the socket when that context fires. Confirm a slow-drip peer cannot hold an in-use turn past that budget — if refreshing the deadline lets a trickling peer pin a turn indefinitely, the fix is worse than the bug.
- `clampTimeout` currently narrows `ioBound` to the remaining context time. Whatever you do must keep the caller's own deadline authoritative, and must keep the existing distinction between a caller deadline (`context.DeadlineExceeded`) and the library budget (`redis:timeout`). There are existing tests for that distinction — do not break them.

Also reconcile `maxBulkLength` with whatever the configured `IOTimeout` can actually carry, so the decoder stops advertising sizes it cannot deliver.

Constraints:
- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (Traefik plugin): no reflection, no `syscall`, no new non-stdlib imports. Existing comment in `resp.go`: `net.Error` interface asserts have panicked under Yaegi and `errors.Is` is used instead — respect that.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code.
- Preserve existing invariants: in-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks.
- Five sibling agents are fixing other `simpleredis` bugs in parallel. BUG-4 (bounded reply allocation) also edits `readBulk` in `resp.go`, so a conflict there is likely. Keep the diff surgical and confined to the deadline concern — do not also change how the payload is allocated.

Regression test: Add a permanent test to the **default** (untagged) suite that fails before the fix and passes after: a compliant fake peer that streams a multi-megabyte bulk value in chunks over a period well beyond `IOTimeout`, asserting the value is returned intact. Add a companion test proving a peer that goes *silent* mid-reply still times out promptly, so the stall bound is real. Prefix any new fake or helper with something bug-specific so it cannot collide with sibling branches.

THE SIMPLICITY GATE (overrides workflow Done when for later phases; prepare still completes): Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, do not implement it. Prepare still grounds the ticket fully.

Also: unattended full run subject to that gate. Only simpleredis.
