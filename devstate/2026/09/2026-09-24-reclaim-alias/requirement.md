# Requirement
IssueKey: 2026-09-24-reclaim-alias

## Problem
The crowdsec bouncer vendors `github.com/david-garcia-garcia/traefik-middleware-utilities` at v1.0.6 but its `reclaim/` tree carries local changes (notably alias support) that are not on `origin/master` of the utilities repo. Those changes need to land on utilities `master` as a PR with measured delta and extensive tests. The bouncer repo itself must not be modified in this run.

## Current (code)
- `origin/master` at `ba52347e91471b136d85e8dae247dee929992e65` (`reclaim/` in this worktree): `reclaim/table.go`, `reclaim/opentyped.go`, and existing reclaim tests only; no `reclaim/alias.go`; `Table` has no `aliases` field; no `Peek` API; slot teardown paths do not call alias unbind helpers.
- Bouncer vendor `reclaim/` (measured vs worktree `origin/master`): adds `reclaim/alias.go`; extends `reclaim/table.go` (~63 net lines); `reclaim/opentyped.go` is byte-identical to `origin/master`.
- New vendor file `reclaim/alias.go`: `SetAlias`, `Watch`, `ClearPublisher`; types `Watcher`, `Box`, `Published`; weak alias watchers; publisher/group rename rules; `unbindIncarnationLocked` / `clearAliasLocked` / `detachAliasLocked` / `deliverLocked` helpers.
- Vendor `reclaim/table.go` delta vs `origin/master`: `Table.aliases map[string]*aliasEntry`; `slot.aliases []*aliasEntry`; `New` initializes `aliases`; new `Peek(key)` with `State` (`Awake` / `Asleep`); `unbindIncarnationLocked(incarnation)` on `endBusySlot`, `unmapLocked`, and `installCloser` failure path; on `expire` `expireDispose` after `dispose`; `takeAll` unbinds per slot and resets `t.aliases`.
- Utilities main checkout untracked/local-only reclaim artifacts (`reclaim/BUGS.md`, scratch repro tests, `.agents`, coverage): not part of the vendor delta; out of scope as source of truth.
- Bouncer pin: `go.mod` references `traefik-middleware-utilities v1.0.6` (caller); compare target is vendor vs this worktree’s `origin/master`, not vs tag alone.

## Desired
- Open a PR against `github.com/david-garcia-garcia/traefik-middleware-utilities` `master` containing exactly the measured vendor reclaim delta (new `alias.go` plus `table.go` changes; no other packages).
- Record and implement that delta faithfully (alias API + `Peek` + alias cleanup on incarnation teardown paths reflected in the diff).
- Add extensive test coverage in the PR for alias behavior, `Peek`, and alias cleanup on unmap/expire/takeAll (caller requirement).
- Do not modify the crowdsec bouncer repository in this run.

## Affected
- `reclaim/alias.go` (add)
- `reclaim/table.go` (modify)
- New/extended tests under `reclaim/` for the proposed behavior

## Out of scope
- `simpleredis`, `iplookup`, or any package outside `reclaim/`
- Untracked files in the utilities main checkout (`reclaim/BUGS.md`, scratch repro tests, `.agents`, coverage)
- Bumping bouncer `go.mod` or editing bouncer vendor tree
- Treating v1.0.6 tag alone as the diff baseline (baseline is vendor vs `origin/master`)

## Unknowns
- Which alias / `Peek` edge cases beyond the vendor implementation need explicit tests (explore/propose).
- Whether upstream wants the vendor file header comment on `alias.go` retained as-is.
- Post-merge tag/release plan for bouncer to consume a new utilities version.

## Tensions
- Requester said “a bug was probably fixed”; measured delta shows alias unbind on several slot teardown paths (`endBusySlot`, `unmapLocked`, `installCloser`, `expire` dispose, `takeAll`) rather than a separate one-line fix — treat all measured hunks as in scope unless explore narrows.
