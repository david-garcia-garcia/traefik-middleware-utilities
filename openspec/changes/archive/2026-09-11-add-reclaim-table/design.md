## Context

DestBranch (`origin/initial`) has no Go module. Source of the table is `david-garcia-garcia/traefik-geoblock` PR #83 @ `22f09a0` (`pkg/reclaim`). Traefik v3.7.11 loads local plugins from `./plugins-local/src/<module>/` via Yaegi v0.16.1; Yaegi does not fetch Go modules. See proposal.md for why. Specs: `std_go_reclaim_context-lease`, `std_go_reclaim_value-lifecycle`. Explore decisions: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Byte-faithful port of geoblock `pkg/reclaim` into `reclaim/` with import-path and package-layout edits only.
- Prove the library under Traefik Yaegi with a fake plugin that is not the library module root.

**Non-Goals:**
- Changing `Open` to explicit lifecycle hooks.
- Vendoring Yaegi into the library `go.mod`.
- Geoblock plugin behavior, Redis, leaky bucket.

## Decisions

1. **Non-generic `Table` of `any`.** Alternative `Table[T]` panics under Yaegi when instantiated from another package. Callers type-assert.

2. **Nested fake plugin, not root `plugin.go`.** This repo is a library. Module `github.com/david-garcia-garcia/reclaimprobe` under `e2e/reclaimprobe/` exports `Config`, `CreateConfig`, `New`. Compose mounts:
   - `./e2e/reclaimprobe` → `plugins-local/src/github.com/david-garcia-garcia/reclaimprobe`
   - `./` → `plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`
   Yaegi GOPATH resolves the reclaim import without vendor. Alternative: Traefik symbols on the library root — rejected so consumers do not import a plugin package.

3. **`traefik:v3.7.11`, `useunsafe: false`.** Matches the measured Yaegi pin. Reclaim is stdlib-only.

4. **Keep `Reset`.** Matches imported specs and geoblock tests.

5. **Pester is the Yaegi proof.** Two whoami routes, one probe middleware alias, shared Open key, headers for value identity, parse Traefik logs for `reclaim_put`/`reclaim_bind`. Unit tests own grace/lifecycle timing. Optional reload recreate if cheap.

6. **Type-switch optional interfaces.** Comma-ok interface assert panics under Yaegi. Hooks stay inert interpreted; compiled tests still require them.

## Risks / Trade-offs

- [Yaegi optional hooks never fire] → Mitigation: e2e does not fail on inert Sleep/Wake/Close; debt file `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md`.
- [GOPATH dual mount fails to resolve reclaim] → Mitigation: keep probe imports to this module's `reclaim` only; no third-party deps.
- [Docker port 8000/8080 clash] → Mitigation: document ports; fail fast on Traefik API wait.
- [Empty dest README drift] → Mitigation: commit the caller README in implement.

## Migration Plan

New library. No production deploy. Rollback is revert the branch. Geoblock can switch imports after this lands; not this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
