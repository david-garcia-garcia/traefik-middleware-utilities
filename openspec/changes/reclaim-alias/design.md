## Context

See `proposal.md` — Why. Baseline is `origin/master` at `ba52347e91471b136d85e8dae247dee929992e65`. Source of truth for production code is the crowdsec bouncer vendor tree at `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim/` (`alias.go` add, `table.go` hunks; `opentyped.go` unchanged). Holders stay strong via `Open`; aliases are weak subscriptions on opaque public names owned by callers.

## Goals / Non-Goals

**Goals:**

- Land the measured vendor delta verbatim in behavior (alias API, `Peek`, teardown unbind wiring).
- Document exported alias types for Yaegi (`Box`, `Published`, `func(any)` hook).
- Prove behavior with a test matrix aligned to bouncer `pkg/reclaim/zzz_alias_test.go` and `zzz_peek_test.go`, plus utilities-only teardown cases.

**Non-Goals:**

- `simpleredis`, `iplookup`, or any package outside `reclaim/`.
- Bouncer `go.mod`, vendor bump, or tag/release.
- Migrating `e2e/reclaimprobe` to alias or `Peek`.
- Copying bouncer shim helpers (`ResetForTest`, `watchInto`) — utilities tests use `NewTable` + `Table.Reset()`.

## Decisions

- **Starting point is the vendor files**, applied to worktree `reclaim/` with the vendor banner rewritten to upstream doc voice (`explore.md` decision).
- **`alias.go` owns alias graph; `table.go` owns incarnation hooks.** `unbindIncarnationLocked` is called from existing teardown sites only — no new public API for unbind.
- **Caller owns alias/publisher/group strings.** Table treats them as opaque; no Traefik instance naming in utilities.
- **FindSpecHost:** fold into `std_go_reclaim_value-lifecycle` (high). Candidate `std_go_reclaim_context-lease` unchanged — bind/`Open` rules do not move; no second delta folder.
- **Tests in-package** beside existing reclaim tests; Yaegi cases only where interpreter constraints matter (`Published` as `func(any)` arg, `Box` in `atomic.Value`), skip under `-race` like siblings.
- **One job, one owner:** alias delivery runs under table lock; `changed` must not re-enter the table (documented panic risk avoided by contract).

## Risks / Trade-offs

- [Miss a teardown path → stale alias points at dead incarnation] → Match vendor call sites exactly; utilities-only tests for unmap, grace expire, `Reset`, busy-slot failure.
- [Second publisher races replacement] → Vendor rejects conflicting publisher; test dying incarnation does not clobber replacement publisher.
- [Peek mistaken for bind] → Tests assert peek does not hold slot awake or shorten grace.
- [Yaegi typed-nil in `atomic.Value`] → Keep `Box` wrapper; optional Yaegi harness mirrors prior reclaim patterns.

## Migration Plan

- Merge to `master` via PR #100. Existing `Open` / `OpenTyped` callers compile unchanged; new APIs are opt-in.
- Downstream re-vendors when a tag exists (out of this run).
- Rollback: revert the PR.

## Open Questions

None.
