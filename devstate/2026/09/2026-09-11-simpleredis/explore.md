# Explore
IssueKey: 2026-09-11-simpleredis

Verdict: in progress

## Concepts

Dest is a **library module**, not a Traefik plugin. Crowdsec-bouncer owns `pkg/simpleredis` as a stdlib RESP client; this ticket copies that client here so other middlewares import it from `github.com/david-garcia-garcia/traefik-middleware-utilities`. Out of scope: crowdsec `pkg/cache`, LAPI, captcha, decisionscope, `go-redis`, miniredis, TLS, Unix sockets.

```
  crowdsec-bouncer @ 6548da47              this repo (dest)
  ────────────────────────────            ────────────────
  plugin.go  (does not import simpleredis)
       │
       └─ pkg/cache  (out of scope)
            └─ pkg/simpleredis  ──────►   simpleredis/
                 simpleredis.go              simpleredis.go
                 simpleredis_test.go         simpleredis_test.go
                                               yaegi_test.go   (new)
```

Pinned facts: `knowledge/research/ext_crowdsec_simpleredis/` (API, RESP, fake-TCP tests, Apache-2.0, Yaegi `ioError` comment). Dest usage packet for this client does not exist yet (`knowledge/devdocs/index_std_go.md` lists only Reclaim). Reclaim usage `knowledge/devdocs/std_go_reclaim.md` is the sibling pattern, not this API. Traefik loader: `knowledge/research/ext_traefik_plugins_local-loader/`. Pester harness: `knowledge/research/ext_geoblock_pester-integration/` plus dest `e2e/reclaimprobe/`.

Yaegi loads GOPATH. Dest already mounts the library tree and a nested fake plugin:

```
  docker Traefik cwd
  plugins-local/src/github.com/david-garcia-garcia/
    reclaimprobe/                         ← existing e2e plugin
    simpleredisprobe/                     ← new e2e plugin (this change)
    traefik-middleware-utilities/
      reclaim/
      simpleredis/                       ← library under test
```

SimpleRedis is stdlib-only (`net.Dialer` TCP RESP). `Init` assigns host/pass/database and does not dial; first Get/Set/Del/MGet dials. After `Close`, commands return `redis:unreachable` without dialing. No generics, no `unsafe`, no cgo — same Yaegi envelope as reclaim (`useunsafe: false`).

Two proof layers, same split dest already uses for reclaim:

| Layer | Owner | Redis |
|---|---|---|
| Compiled `go test` | copied `simpleredis_test.go` | in-process `startFakeRedis` / `startStaticRedis` |
| Yaegi `go test` | `simpleredis/yaegi_test.go` | compiled fake listener; interpreted client |
| Pester + Traefik | `e2e/simpleredisprobe` | compose `redis` container |

Dest gap (not a crash): `simpleredis/` and `redis/` are absent on this worktree; `go.mod` requires only Yaegi; README Libraries row is Planned; `docker-compose.yml` has no Redis service. Searched worktree globs `{redis,simpleredis}/**` — 0 files.

No identity reconstruction (client address, user, tenant, request Host). `Init`'s host is the Redis server address from caller config.

## Decisions

**Land the client as package `simpleredis` under `simpleredis/`.** Dest sibling `reclaim/` kept the source package name and `folder == package`. README Layout `redis/` is a Planned placeholder written before this source was chosen. Import last segment stays `simpleredis`. Exported type `SimpleRedis`, `Init`/`Get`/`MGet`/`Set`/`Del`/`Close`, and error string constants copy as-is (crowdsec cache matches those strings). Change the module prefix only. Do not rename the package clause to `redis`.

**Keep dest's English library row, but name the type.** README Libraries title becomes SimpleRedis (the exported type). Role: shared stdlib RESP client (GET/MGET/SET/DEL) middlewares import instead of inventing one. Status Current. Layout block `redis/` → `simpleredis/`. Tests block adds `go test ./simpleredis/...`.

**Copy existing tests, then add dest Yaegi tests.** Unit coverage stays the in-process fake TCP suite (no live Redis, no miniredis). `simpleredis/yaegi_test.go` mirrors `reclaim/yaegi_test.go`: GOPATH copy of non-test sources, `interp` with `stdlib` only, `useunsafe` false. Compiled test owns the listener; interpreted probe calls `Init`/`Get`/`Set`/`Del`.

**Host plugin is a nested module, not root `plugin.go`.** Same reason as reclaim: this repo is a library. `e2e/simpleredisprobe/` — module `github.com/david-garcia-garcia/simpleredisprobe`, replace to repo root, `.traefik.yml`, `Config`/`CreateConfig`/`New`. `New` calls `Init` only (no dial, so Traefik still starts if Redis is late). `ServeHTTP` SET+GET against compose Redis and sets a response header Pester can assert. `useunsafe: false`.

**Share the existing compose project.** Desired 3 says follow dest reclaim (`docker-compose.yml`, `Test-Integration.ps1`, one Pester file). Out of scope allows harness share without changing reclaim semantics. Add: `redis` service, second `localPlugins` entry, mount for `simpleredisprobe`, a whoami that is not `whoami-a`/`whoami-b`. Keep project name `reclaim-e2e`, container `reclaim-e2e-traefik`, ports 8000/8080, routes `/a` `/b`, Traefik `v3.7.11`. Traefik already iterates each `localPlugins` entry (`ext_traefik_plugins_local-loader`).

Harness call sites (searched `docker-compose.yml`, `Test-Integration.ps1`, `scripts/`, `.github/workflows/`, `README.md`, `e2e/`): 4 product files — `Test-Integration.ps1` (compose up, health 8080 and `/a` `/b`, logs `reclaim-e2e-traefik`), `scripts/integration-tests.Tests.ps1` (`reclaim-e2e-traefik`, stop/start `whoami-a`/`whoami-b`), `.github/workflows/ci.yml` (`./Test-Integration.ps1`, compose logs traefik/whoami-a/b, compose down), `README.md` Tests. Sharing by **adding** services does not migrate those reclaim paths. Pester redis Describe must not stop `whoami-a`/`whoami-b`.

**Pester Redis is a real compose container.** Copied unit tests already fake RESP. E2e is Traefik+Yaegi+network; a fake inside the plugin would not prove dial. Image `redis:7-alpine`, hostname `redis:6379` (crowdsec default), no password, empty database. AUTH/SELECT stay unit-tested. Probe Config default host `redis:6379`.

**One integration runner.** CI has one integration job calling `./Test-Integration.ps1`. Add Redis + `/redis` health waits and a second Describe. Do not split scripts or jobs. Failure logs also dump `redis` and the new whoami.

**Apache-2.0 stays on the copied package, not a product LICENSE.** Dest has no root `LICENSE` (glob `LICENSE*` at repo root: not found). Adding one would relicense `reclaim/` without a criterion. Ship `simpleredis/LICENSE` (Apache-2.0 text plus the source appendix copyrights: Containous SAS, Traefik Labs). README states that copied client is Apache-2.0. No `go.mod` Redis require (research: stdlib only).

**Specs/devdocs wait for propose/implement.** No dest SimpleRedis packet or spec leaf yet. Explore did not invent Language. Propose adds `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands`. Implement/devdocsimpact write usage once the package exists.

## Open questions

- Q: Folder and import last segment — README `redis/` vs source package `simpleredis`?
  Rank: additive asked — new library this change creates; Desired 1 names the simpleredis copy so callers import it from this module; dest sibling `reclaim/` is folder=package
  Decision: assumed — `simpleredis/` and `package simpleredis`. README Layout `redis/` becomes `simpleredis/`. Do not change the package clause to `redis`.
  By: propose

- Q: Ticket names simpleredis vs README “Redis connection” under `redis/`?
  Rank: additive asked — README Libraries/Layout are in Affected and Desired 4
  Decision: assumed — Libraries title SimpleRedis (exported type); Role is the shared stdlib RESP client; Status Current. “Redis connection” was the Planned placeholder.
  By: propose

- Q: Does the host plugin share the reclaim compose project or get its own stack/ports?
  Rank: additive asked — new probe + compose services; Desired 3 names dest reclaim e2e (one compose, one runner); Out of scope allows sharing the harness
  Decision: assumed — share `docker-compose.yml` project `reclaim-e2e`. Add redis + `simpleredisprobe` + `whoami-redis` on `/redis`. Do not change project name, `reclaim-e2e-traefik`, ports 8000/8080, or reclaim `/a` `/b`.
  By: propose

- Q: Is e2e Redis a compose `redis` container or a fake like `startFakeRedis`?
  Rank: additive asked — new compose service; Desired 3 host plugin uses Redis; Desired 1 copies existing fake-TCP unit tests; Out of scope forbids miniredis
  Decision: assumed — Pester uses compose `redis:7-alpine` at `redis:6379` with no password and empty database. Copied unit tests and Yaegi go tests keep in-process fake TCP. No miniredis, no TLS, no Unix sockets.
  By: propose

- Q: How does Apache-2.0 attribution land given dest has no root LICENSE?
  Rank: additive incidental — NOTICE/LICENSE on files this change creates; Affected says “Possibly root license/NOTICE”; no criterion names a product-wide LICENSE
  Decision: assumed — package-local `simpleredis/LICENSE` (Apache-2.0 + Containous SAS / Traefik Labs copyrights from source appendix). Do not add a repo-root LICENSE (would relicense reclaim). README notes the copied client is Apache-2.0.
  By: propose

- Q: Does `Test-Integration.ps1` stay one script for both libraries or split?
  Rank: additive asked — extend the existing runner; Desired 3 following dest reclaim; CI integration job calls that one script
  Decision: assumed — one `Test-Integration.ps1` and one `scripts/integration-tests.Tests.ps1`. Add Redis and `/redis` waits plus a Describe that does not stop whoami-a/b. Do not split scripts or CI jobs.
  By: propose

- Q: What is the fake middleware shape for Redis Yaegi e2e?
  Rank: additive asked — Desired 3 following dest `e2e/reclaimprobe/`
  Decision: assumed — nested module `e2e/simpleredisprobe` (package `simpleredisprobe`) with Traefik `Config` / `CreateConfig` / `New`; `New` Inits the client (host from Config, default `redis:6379`); handler SET+GET plus response header `X-SimpleRedis-Value`; GOPATH mounts plugin + this repo; no root `plugin.go`; `useunsafe: false`.
  By: propose

- Q: Where do Yaegi interpreter tests live?
  Rank: additive asked — Desired 2 names Yaegi-specific tests; dest pattern `reclaim/yaegi_test.go`
  Decision: assumed — `simpleredis/yaegi_test.go`. GOPATH copy of non-test sources; interp stdlib only. Compiled test owns the fake TCP listener; interpreted probe exercises Init/Get/Set/Del.
  By: propose

- Q: Who already owns client address / user / tenant / Host for this client?
  Rank: additive asked — explore requires an owner when identity is in play; this unit does not reconstruct request identity
  Decision: resolved — none. `Init` host is the Redis server address from caller config. The probe does not read `RemoteAddr` or forwarded headers. Traefik still owns plugin `New` ctx.
  By: propose

- Q: Does dest `go.mod` need a Redis client module?
  Rank: additive asked — dest `go.mod` currently requires only Yaegi; research: source client is stdlib TCP RESP
  Decision: resolved — no new require. Probe `go.mod` replace-imports this module only. Do not add Yaegi to the probe module.
  By: propose
