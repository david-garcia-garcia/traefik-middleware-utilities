## Why

On DestBranch a Traefik middleware that fronts a backend has no reusable admission gate: each plugin forwards into a dying upstream or reinvents an ad-hoc retry. Traefik's CircuitBreaker is an HTTP middleware, not a Yaegi-safe library, and it has no exponential backoff.

## What Changes

- New package `backendbackoff/`: in-memory per-key Gate. Caller reports a boolean outcome of a real backend attempt. The library does not read HTTP, classify failures, sleep, or write a response.
- Saturating success-credit trip (`p`, `B`) independent of time. Exponential jittered cooldown independent of outcomes. States CLOSED / OPEN / HALF-OPEN. Probe success restores credit and retains `n`. `n` resets after one MaxCooldown of continuous CLOSED.
- Bounded map (TTL eviction, cap 65536, `dropExpired` / `dropOne` shape). Do not import `tokenbucket`. No Redis, no goroutine. `Close` at most. Storable in existing `reclaim.Open`.
- `go test -short` plus interpreted Yaegi test and `TestAlloc*` on Allow. No `*_LIVE_*`, no e2e-redis/dragonfly, no Pester plugin.
- README: new section, Layout row, one line in Why this exists so the module is not described as Redis-only.
- New spec family `std_go_backendbackoff` and usage packet `knowledge/devdocs/std_go_backendbackoff.md`.

## Capabilities

### New Capabilities

- `std_go_backendbackoff_allow`: Allow/Report, saturating credit trip, opaque caller-owned key, idle TTL map, construction defaults and validation, Yaegi and alloc proof.
- `std_go_backendbackoff_cooldown`: OPEN / HALF-OPEN, exponential jittered cooldown, `n` retain and reset, single outstanding probe and lost-probe lease.

### Modified Capabilities

- None. `std_go_tokenbucket_allow` is a different clock. `std_go_ci_test-suites` is unchanged: this package has no LIVE env and is not added to e2e jobs. `std_go_reclaim_*` is unchanged: callers pass `*Gate` to existing `Open`.

## Impact

- New `backendbackoff/` (stdlib only).
- `README.md` (`## Why this exists`, `## Layout`, new section).
- `openspec/specs/domains.md` unchanged (`std` / `go` already allowed). `openspec/specs/map.md` regenerated after archive.
- `knowledge/devdocs/std_go_backendbackoff.md` and `index_std_go.md`.
- `knowledge/debt/2026-09-12-backendbackoff-shared-layer.md` (deferred shared layer).
- No change to `simpleredis/`, `windowcounter/`, `tokenbucket/`, `reclaim/`, or `.github/workflows/ci.yml` e2e jobs.
