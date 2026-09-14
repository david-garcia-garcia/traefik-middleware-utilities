TEST-ONLY change. Do not modify `reclaim/table.go`.

`reclaim` sits at 94.4% of statements with NINE blocks that no test in the package ever executed, including two behaviours `knowledge/devdocs/std_go_reclaim.md` explicitly promises. The caller wrote seven tests that take it to 100.0% and verified they pass against `master`. This ticket lands them properly through the workflow, including the spec and devdocs impact phases, because two of them pin documented contracts that had no test.

Copy verbatim into `reclaim/` in your worktree (it is untracked in the main checkout, so a fresh worktree will not have it) DURING IMPLEMENT, not during prepare product commits: `D:/repositories/traefik-middleware-utilities/reclaim/table_gaps_test.go`. Prepare must not land the test file; prepare's first commit is the empty `chore: start <IssueKey>` if origin/master...HEAD is empty.

Read-only reference (MAIN checkout, currently untracked). Read for context. DO NOT commit, delete, move or edit them:
- `D:/repositories/traefik-middleware-utilities/knowledge/debt/2026-09-14-reclaim-race-leak-audit.md` — Finding 3 is this ticket.
- `D:/repositories/traefik-middleware-utilities/reclaim/BUGS.md` — prior bug hunt.

The nine blocks and why nothing reached them (analysis for later explore/spec; record the facts in requirement.md Current/Desired):
1. `waitCtx`'s `ctx.Done() != nil` branch (3 blocks) is UNREACHABLE from the table. `watch` is its only caller and `dropWhenDone` starts `watch` only when `ctx.Done()` is nil. Covered by calling `waitCtx` directly.
2. `waitCtx`'s non-blocking check at the top of the polling loop duplicates the blocking select below it, and only wins when both channels are ready and the select happens to pick the ticker. Covered by calling `waitCtx` with an already-closed `finished`.
3. `Open`'s `case slotGone` needs a slot that is mapped AND already gone. Every ending path unmaps or replaces the key inside the same `t.mu` hold, EXCEPT `Reset`, which swaps the items map and only then ends each slot: a `put` that maps its slot back in between leaves exactly that shape. Not reliably schedulable, so the test builds the shape.
4. `drop`'s busy-wait loop (`for incarnation.state == slotBusy`) is not reachable under any ordinary schedule: a busy slot only ever has holders that have not bound yet, so no pending drop can meet one.
5. `expire`'s early return is the armed `AfterFunc` arriving after an `Open` already woke the value, or after another path claimed it.
6. `Reset` with a panicking `Sleep` hook: devdocs says it skips orphan and still disposes. Never tested.
7. `Reset` with `EnforceCloseBeforeOpen`: devdocs says Reset unmaps first regardless of the flag, so a later `Open` creates instead of waiting for that `Close`. Never tested.

Items 1 and 4 look like genuinely dead defensive code, and item 2 is a redundant fast path. Do NOT delete them unilaterally. Later phases write `devstate/issues.md` rows proposing either deletion or an explicit comment that they are unreachable, sized per the opd-workflow Issues table (these are `large` / `note`, so they need a `knowledge/debt/` file). Prepare records this as unknowns/out of scope as appropriate — do not take deletion in this ticket.

Tests for items 3, 4 and 5 are white-box: they take `tab.mu` and construct `slot` state directly. Keep them that way.

A second subagent is running `/opd-workflow` in parallel on `2026-09-14-reclaim-bug-finished-race-lock-defer`, branched from the same `master`. It rewrites locking in `reclaim/table.go`. It adds only other test files, so no textual conflict with `table_gaps_test.go`. Because the tests touch `tab.mu` and `slot` internals directly, a rename on that branch could break these at merge time. Flag the coupling in requirement.md Tensions / Unknowns; do not try to coordinate.

Acceptance for later implement (do not run the tests in prepare unless needed to ground Current):
- `go test -count=1 -timeout 10m ./reclaim/` green.
- Docker race: `docker run --rm -v "<worktree>:/src" -w /src golang:1.25 go test -race -count=1 -timeout 10m ./reclaim/`
- `reclaim` statement coverage exactly 100.0%, zero uncovered blocks.
- `reclaim/table.go` byte-identical to `master`.
- Stub PR opened during prepare on GitHub, CI measured later.
