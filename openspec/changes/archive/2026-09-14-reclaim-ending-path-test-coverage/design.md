## Context

See proposal.md for why. Dest `reclaim/table.go` already implements the two Reset contracts. Package tests on `origin/master` leave nine statement blocks at count 0 (measured 94.4%). Caller-authored `reclaim/table_gaps_test.go` (untracked in the main checkout) covers them. Usage packet `knowledge/devdocs/std_go_reclaim.md` already states both Reset contracts.

Explore decisions: land the file verbatim; do not edit `table.go`; fold scenarios into `std_go_reclaim_value-lifecycle` and `std_go_reclaim_context-lease`; note unreachable defenses as debt.

## Goals / Non-Goals

**Goals:**
- Copy `reclaim/table_gaps_test.go` into the package unchanged.
- `go test ./reclaim/` green; Docker `go test -race ./reclaim/` green; statement coverage 100.0% with zero count-0 profile rows.
- `git diff origin/master -- reclaim/table.go` empty.

**Non-Goals:**
- Edit `reclaim/table.go`.
- Delete or comment the unreachable `waitCtx` Done branch, `drop` busy-wait, or redundant poll fast path.
- Coordinate with `2026-09-14-reclaim-bug-finished-race-lock-defer`.
- New fakes. The tests reuse instruments already in `reclaim/table_test.go` and `releaseOnce` from `reclaim/repro_enforce_panic_close_test.go`.

## Decisions

### Verbatim copy, in-package white-box
Copy the file from `D:/repositories/traefik-middleware-utilities/reclaim/table_gaps_test.go`. Keep `package reclaim` so tests can call `waitCtx` and take `tab.mu`. That matches dest `mustSlot` / `readState`.

Alternative considered: rewrite the three race-shaped tests as public-API schedules. Rejected: those windows are not reliably schedulable (explore).

### Spec tightens MAY to SHALL for Reset unmap
Dest already unmaps first. Live specs said MAY. This change's tests pin the exception, so the delta uses SHALL and adds scenarios. Sleep-panic skip-orphan is the same: dest already does it; the log requirement's "no exception for Reset" is the line that changes.

Alternative considered: `skip_specs` because production behaviour is unchanged. Rejected: the ticket is to pin documented contracts that had no test, and archive needs the scenarios on the live leaves.

### Coverage proof is a profile, not a CI gate
CI has no statement-coverage job. Implement measures `go tool cover -func` plus count-0 rows locally, and Docker `-race` because this host cannot enable cgo.

## Risks / Trade-offs

- [Risk] Sibling locking PR renames `tab.mu` or `slot` fields → Mitigation: debt file `knowledge/debt/2026-09-14-reclaim-whitebox-test-lock-coupling.md`; this change does not coordinate.
- [Risk] White-box tests lock `tab.mu` while production code also locks it → Mitigation: they set state under the lock then call the method, same as existing helpers; Docker `-race` is the proof.
- [Trade-off] Unreachable defenses stay. Required: test-only change; deletion is a later ticket.

## Migration Plan

Test-only. Rollback is revert. No stored data. No caller API change.
