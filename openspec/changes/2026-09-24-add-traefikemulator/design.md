## Context

See `proposal.md` — Why. On `master` there is no `traefikemulator/` directory. The bouncer repo already ships a 105-line stdlib helper and six unit tests. Sibling packages (`reclaim`, `iplookup`) live as top-level dirs with in-package white-box tests and no `pkg/` prefix. CI unit jobs already run `go test -short ./...` and `-race -short`.

## Goals / Non-Goals

**Goals:**

- Copy `emulator.go` verbatim in behavior from bouncer `pkg/traefikemulator`.
- Port all six existing tests into `emulator_test.go` (in-package `traefikemulator`, not `traefikemulator_test`).
- Add unit tests for uncovered branches listed in `explore.md` until `go test -cover ./traefikemulator/` reaches ~95%+ statement coverage (same band as `iplookup`).
- Register one new live spec leaf for generation semantics.

**Non-Goals:**

- Switching bouncer imports to this module path.
- Yaegi tests, Go E2E, Pester, or Traefik Docker integration for this helper.
- Usage packet `knowledge/devdocs/std_go_traefikemulator.md` (devdocsimpact).
- Changing `.github/workflows/ci.yml` (new package is included by existing `./...` jobs).

## Decisions

1. **Top-level `traefikemulator/`** — Matches `reclaim/` and `iplookup/`. Rejected: `pkg/traefikemulator` (not used on this module).

2. **FindSpecHost** — Search: `openspec/specs/map.md` (no `traefikemulator` family), live leaves under `std_go_*`. **Verdict: new** `std_go_traefikemulator_generation`. No fold target; this is net-new published API behavior.

3. **Single spec leaf** — One capability covers cancel, shared context, partial failure, duplicate names, Stop, and Serve/Handler. Rejected: splitting routing vs generation (one cohesive surface, ~100 LOC).

4. **Test layout** — `emulator_test.go` only for unit proofs. Rejected: `traefikemulator_yaegi_test.go` (helper is compiled-test-only; no plugin entrypoint under Yaegi).

5. **Coverage bar** — Port six bouncer tests first, then add: duplicate route in one `Apply`; `Serve`/`Handler` false for missing route and after `Stop`; `Stop` cancels contexts; empty `Apply` if needed for map reset; document or test `New(nil)` panic (only branch that may stay below 100%).

6. **README** — Add a short section plus Layout row like `iplookup` (test helper, stdlib, no LIVE env).

## Risks / Trade-offs

- **[Risk] Drift from bouncer copy** → Mitigation: port files directly; behavioral tests lock semantics.
- **[Risk] Over-testing panic branch** → Mitigation: accept ~95% if only `New(nil)` panic remains uncovered; note in implement if so.

## Migration Plan

New package only. No runtime migration. Downstream repos may adopt the import path in follow-up PRs (out of scope here).
