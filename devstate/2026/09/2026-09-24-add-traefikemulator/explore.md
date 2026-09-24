# Explore
IssueKey: 2026-09-24-add-traefikemulator

## Concepts

Upstream the bouncer’s in-repo test helper so Traefik plugin authors can depend on `github.com/david-garcia-garcia/traefik-middleware-utilities/traefikemulator` instead of copying `pkg/traefikemulator`.

```
  test (compiled go test)
        |
        v
  traefikemulator.New(Constructor)
        |
        v
  Apply([]Route) --cancel prior ctx--> one shared context per generation
        |                                      |
        |                                      v
        +--> map[name]http.Handler <---- plugin New(ctx, next, cfg, middlewareName)
        |
        v
  Serve(name, w, req) / Handler(name)
```

- **Emulator** (`emulator.go`): stand-in for Traefik `RouterFactory.CreateRouters`. One generation at a time; `Apply` cancels the previous context, builds handlers in order, returns per-route constructor errors without canceling siblings.
- **Route / Constructor**: test-supplied plugin `New` signature; `MiddlewareName` defaults to `Name`.
- **Sibling layout**: top-level package dirs (`reclaim/`, `iplookup/`, …). No `pkg/` on `origin/master`. Module `github.com/david-garcia-garcia/traefik-middleware-utilities`, Go 1.21.
- **Source reference** (read-only): `crowdsec-bouncer-traefik-plugin/pkg/traefikemulator/emulator.go` (105 lines) and `zzz_emulator_test.go` (six tests). Bouncer integration tests in `zzz_traefikemulator_test.go` stay on the local copy (out of scope).

### Call sites

| Root searched | Count | Role |
| --- | ---: | --- |
| Worktree `*.go` for `traefikemulator` | 0 | Package absent on branch |
| Bouncer `pkg/traefikemulator`, `zzz_traefikemulator_test.go` | 1 package + 1 consumer test file | Copy source; switch import out of scope |

No existing callers in the utilities repo to migrate.

### Reproduce

No failure claimed. **Not reproduced** — this run adds missing published API, not a bugfix.

Measured (throwaway module, bouncer tests only): `go test -cover` → **90.0%** on `emulator.go`. Siblings on this branch: `reclaim` **99.6%**, `iplookup` **94.7%** (`go test -short ./reclaim/... ./iplookup/... -coverprofile=nul`).

Uncovered branches with contributed tests alone: duplicate route name in one `Apply`, `New(nil)` panic, `Handler`/`Serve` on missing route, `Stop` after a generation (handlers cleared), optional empty `Apply`.

### CI / test suites (in-tree)

From `knowledge/devdocs/std_go_test-suites.md`: Unit job runs `go test -short ./...`; in-package white-box tests; Yaegi files where plugin code must run under the interpreter. No Redis/E2E/Pester for packages that are pure in-process helpers (`iplookup`/`reclaim` Yaegi prove interpreter behavior of **production** APIs).

Outside facts: bouncer `docs/traefikemulator.md` describes usage; not copied in this ticket.

## Decisions

- Add **`traefikemulator/`** at module root with `emulator.go` ported from bouncer (package comment and exports unchanged except module path in imports — none in production file).
- Port **`zzz_emulator_test.go`** → **`emulator_test.go`** (drop `zzz_` prefix; matches `table_test.go`, `helper_test.go` naming). Keep `package traefikemulator` (not `foo_test`; `.golangci.yml` / devdocs: in-package white-box).
- **Implement phase test plan**: run `go test -cover ./traefikemulator/` after port; add unit tests until statement coverage is in the same band as `iplookup` (~95%+) without inventing repro/race files unless a real concurrency hazard appears (none observed — single goroutine test helper).
  - Required additions (close known gaps): duplicate route in one `Apply`; `Serve`/`Handler` false when route omitted or after `Stop`; `Stop` cancels generation contexts; optionally `New(nil)` panic via `defer recover` or subtest documented as panic test.
  - **No** `traefikemulator_yaegi_test.go`: helper uses only stdlib; no Yaegi-specific surface (contrast `reclaim/yaegi_test.go`, `iplookup/helper_yaegi_test.go` proving interpreted plugin code).
  - **No** `*_e2e_test.go`, Pester, or integration scripts — nothing live to talk to.
  - CI: new package is picked up automatically by existing `test` and `race` jobs (`go test -short ./...` / `-race -short`).
- **OpenSpec**: **no live contract** for `traefikemulator` today (`openspec/specs/map.md` has no family). Propose may add `std_go_traefikemulator_*` leaves or record `none — no live contract` if the change is test-helper-only with no SHALL surface; prefer at least one leaf for generation lifecycle if the librarian expects every new top-level package to register.
- **Devdocs**: defer **`knowledge/devdocs/std_go_traefikemulator.md`** to devdocsimpact (usage: how plugin tests wire `New` + `Apply` + `Serve`); consume `std_go_test-suites.md` for where tests land.
- Rejected: **`pkg/traefikemulator`** (tree convention is top-level dirs). Rejected: **vendoring from bouncer** (explicit upstream copy). Rejected: **bouncer import switch** (out of scope).

## Open questions

- Q: What extra tests beyond the six bouncer cases are required to match reclaim/iplookup depth?
  Rank: additive asked — requirement Desired: “contributed tests plus any additional cases needed so coverage matches sibling packages”
  Decision: assumed — port all six tests; then add focused unit tests for duplicate route, missing `Serve`/`Handler`, and `Stop` lifecycle; run `go test -cover ./traefikemulator/` in implement and stop when coverage ≥ ~95% or uncovered lines are only the `New(nil)` panic branch (document if left untested). No repro_* or race-detector suite unless implement finds a data race (none expected).
  By: explore

- Q: Are Yaegi-specific tests required for traefikemulator?
  Rank: additive asked — requirement Unknowns name this; siblings use `*_yaegi_*` for interpreter-loaded plugin code
  Decision: assumed — no Yaegi tests. The package is only invoked from compiled `go test`; it does not ship plugin entrypoints. Sibling Yaegi files prove production middleware under Yaegi, not test harnesses.
  By: explore

- Q: Should OpenSpec gain new `std_go_traefikemulator_*` specs?
  Rank: additive incidental — new package; requirement does not name OpenSpec ids
  Decision: assumed — propose adds minimal specs for generation cancel/shared context and partial failure semantics **or** records `none — no live contract` with rationale in `specs.md` if the librarian treats helpers as non-contract. Default lean: one leaf `std_go_traefikemulator_generation` mirroring the six behavioral tests unless propose finds catalog rules forbid helper specs.
  By: explore

- Q: Who owns the plugin constructor and request identity passed into `Serve`?
  Rank: additive asked — One job, one owner for facts on the request path
  Decision: assumed — **tests own both**. `Constructor` is injected by the test; `Serve` forwards the test’s `*http.Request` unchanged. Emulator does not parse client IP, Host, or headers.
  By: explore
