# Explore
IssueKey: 2026-09-11-reclaim-table

## Concepts

This repo is a **library**, not a Traefik plugin. Geoblock is a plugin that happens to contain `pkg/reclaim`. The spin-off copies the table, not the plugin surface.

```
  geoblock PR #83                         this repo (dest)
  ─────────────────                       ────────────────
  plugin.go  (Traefik New)                (no plugin at module root)
       │
       ├─ pkg/geoblock   (out of scope)
       └─ pkg/reclaim  ──────────────►   reclaim/
            table.go                       table.go
            default.go                     default.go
            table_test.go                  table_test.go
```

Yaegi loads **GOPATH**, not modules. Traefik `GoPath` is `./plugins-local/`. Third-party imports must be vendored; in-tree subpackages resolve when the interpreted root imports them. Reclaim is stdlib-only, so the fake plugin needs no vendor if GOPATH has both trees.

```
  docker Traefik cwd
  plugins-local/src/
    github.com/david-garcia-garcia/
      reclaimprobe/          ← e2e fake plugin (CreateConfig, New)
      traefik-middleware-utilities/reclaim/  ← library under test
```

Traefik `New(ctx, …)` is the holder context. A config reload cancels that ctx, then calls `New` again. That is the production `Open` / sleep / wake path. Compiled `go test` never sees Yaegi.

## Decisions

**Port the geoblock table as a non-generic `reclaim` package.** `Table` stores `any`. `Open(ctx, key, logger, create func() (any, error))`. Process `Default()` / package `Open`. `NewTable(grace)`. Stdlib imports only. Module `github.com/david-garcia-garcia/traefik-middleware-utilities`, layout `reclaim/` (not `pkg/reclaim/`). Go 1.21 to match geoblock @ `22f09a0`.

**Keep the Open signature and optional `Sleep`/`Wake`/`Close` interfaces.** This ticket is a spin-off for reuse, not an API fork. Changing `Open` to explicit hooks would diverge from the specs we are bringing in and from geoblock callers that will switch imports. Yaegi still strips methods on an interpreted `func() (any, error)` return; that is inherited debt, not a silent API change.

**Fake middleware is a nested GOPATH module, not a root `plugin.go`.** Geoblock puts Traefik symbols on the module root because it *is* a plugin. This repo is a library. Root `plugin.go` + `package traefik_middleware_utilities` would make every consumer of the module look like a plugin. `e2e/reclaimprobe/` is module `github.com/david-garcia-garcia/reclaimprobe`, package `reclaimprobe`, with `Config`, `CreateConfig`, `New`. Compose mounts both trees under `plugins-local/src/`.

**Pester + Docker Traefik is the Yaegi e2e.** New compose (do not copy geoblock's geoblock routes). Image `traefik:v3.7.11`. `useunsafe: false` (reclaim is stdlib). Runner `Test-Integration.ps1` + `scripts/integration-tests.Tests.ps1` following geoblock's wait/Pester/down shape. Two whoami routes sharing one probe middleware (two `New`, same key → same incarnation). Parse Traefik logs for `reclaim_put` / `reclaim_bind`. One reload case if a whoami recreate is cheap (geoblock `/reclaim*` pattern). Grace/dispose timing stays in unit tests unless the probe exposes a short grace without forking `Default()`.

**Do not add Yaegi to the library `go.mod`.** An in-process interp probe is a rank-up, not this ticket. The asked proof is Pester through Traefik.

**Bring in specs `std_go_reclaim_context-lease` and `std_go_reclaim_value-lifecycle`.** Adapt `pkg/reclaim` → `reclaim/`. Skip `core_geoblock_*`. Usage packet `knowledge/devdocs/std_go_reclaim.md` adapted from geoblock.

**Keep exported `Reset`.**** Specs and geoblock tests name it. A rename would fork the test seam geoblock will import. Comment stays "tests only".

**Commit the caller README** into the product tree during implement (it is the dest layout contract; it is not on `origin/initial`).

Measured dest failure: `git ls-tree origin/initial` is empty (`ef7be38`). No runtime to reproduce. The gap is absence of `reclaim/`, not a crash.

## Open questions

- Q: What is the fake middleware shape for Yaegi e2e?
  Decision: assumed — nested module `e2e/reclaimprobe` (package `reclaimprobe`) with Traefik `Config` / `CreateConfig` / `New`; `New` calls `reclaim.Open`; handler is passthrough plus reclaim identity headers; GOPATH mounts plugin + this repo; no root `plugin.go`.
  By: explore

- Q: How much of geoblock's Pester/docker harness do we reuse?
  Decision: assumed — reuse the *pattern* (`traefik:v3.7.11`, whoami, API wait, Pester, log helpers for `reclaim_*` msgs). Do not copy geoblock plugin config, GeoIP databases, or geoblock-specific routes.
  By: explore

- Q: Do e2e tests run only on this host via Docker, or also CI?
  Decision: assumed — run on this host with Docker (human). Add a compose + Pester script so a later CI job can call the same runner; this local run does not push, so no remote CI this ticket.
  By: explore

- Q: Under Yaegi, optional `Sleep`/`Wake`/`Close` are inert. Change `Open` to explicit hooks in this new repo?
  Decision: assumed — no. Port the geoblock API and specs. Record the interpreter limitation as follow-up debt. E2e must prove load + `Open` + shared incarnation; inert hooks are expected, not a failing test.
  By: explore

- Q: Traefik image and `useunsafe`?
  Decision: assumed — `traefik:v3.7.11` (Yaegi v0.16.1, same pin as geoblock PR #83). `useunsafe: false`.
  By: explore

- Q: Go module version?
  Decision: assumed — `go 1.21`, matching geoblock @ `22f09a0`.
  By: explore

- Q: Who already owns client address / user / tenant / Host for this table?
  Decision: resolved — none. The table stores caller keys and `any` values. It does not reconstruct request identity. Holders pass Traefik `New` ctx; that ctx is Traefik's.
  By: explore

- Q: Rename test-only `Reset` to `ResetForTest` per commandments?
  Decision: assumed — keep `Reset` so imported specs and geoblock call sites stay aligned.
  By: explore
