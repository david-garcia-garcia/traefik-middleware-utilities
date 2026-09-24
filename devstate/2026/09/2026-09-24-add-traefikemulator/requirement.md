# Requirement
IssueKey: 2026-09-24-add-traefikemulator

## Problem
The traefik-middleware-utilities module has no published `traefikemulator` package on `origin/master`, yet downstream (crowdsec bouncer) already implements Traefik plugin tests against a local copy under `pkg/traefikemulator`. The ask is to upstream that package, open a PR to `master`, and match sibling packages on layout and test depth.

## Current (code)
- `traefikemulator/` at repo root — **not found** on `origin/master` (worktree at `ba52347e`; no directory under module root).
- Top-level packages — `reclaim/` (`reclaim/alias.go`, multiple `*_test.go` including repro and yaegi tests); `iplookup/` (`iplookup/tree.go`, `helper.go`, `*_test.go` including yaegi).
- Module path — `go.mod` declares `github.com/david-garcia-garcia/traefik-middleware-utilities`; packages live as top-level dirs, not under `pkg/`.
- Source to contribute (read-only reference) — `d:\repositories\crowdsec-bouncer-traefik-plugin\pkg\traefikemulator\emulator.go` (`Emulator`, `Apply`, `Stop`, `Handler`, `Serve`; stand-in for Traefik `RouterFactory.CreateRouters`).
- Contributed tests reference — same bouncer path `zzz_emulator_test.go` (six `TestApply_*` / `TestServe_*` cases covering generation cancel, shared context, omitted routes, constructor errors, Serve routing, shared middleware names).
- Bouncer consumption — `crowdsec-bouncer-traefik-plugin` imports `github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/pkg/traefikemulator` in `zzz_traefikemulator_test.go` (local copy; switching import to upstream module version is **out of scope**).

## Desired
- Add new `traefikemulator` package to `github.com/david-garcia-garcia/traefik-middleware-utilities` and open a pull request against `master`.
- Bring implementation from bouncer `pkg/traefikemulator` (`emulator.go`, `zzz_emulator_test.go`) into this repo.
- Place package top-level beside `reclaim` and `iplookup` (no new `pkg/` folder unless the tree already uses it — it does not).
- PR test coverage: contributed tests plus any additional cases needed so coverage matches sibling packages (reclaim/iplookup depth: unit tests, edge/repro-style cases where appropriate).

## Affected
- New directory `traefikemulator/` at module root (`emulator.go`, tests).
- PR #99 (branch `2026-09-24-add-traefikemulator` → `master`).

## Out of scope
- Changes in the crowdsec bouncer repository.
- Copying untracked scratch from main checkout `d:\repositories\traefik-middleware-utilities` (`.agents`, `BUGS.md`, scratch tests, coverage).
- Updating bouncer to depend on upstream `traefikemulator` instead of local `pkg/traefikemulator`.

## Unknowns
- Exact extra test cases beyond bouncer’s six tests needed to match reclaim/iplookup bar (explore/implement measure against `-cover` and sibling patterns).
- Whether yaegi-specific tests are required for this package (siblings use `yaegi_*` where plugins run under yaegi; traefikemulator is a pure test helper — TBD in explore).

## Tensions
- None between caller spec and measured tree.
