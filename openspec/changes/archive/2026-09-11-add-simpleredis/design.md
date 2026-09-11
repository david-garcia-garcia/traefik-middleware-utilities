## Context

DestBranch (`origin/master`) already has module `github.com/david-garcia-garcia/traefik-middleware-utilities`, package `reclaim/`, and a Traefik v3.7.11 Pester harness (`e2e/reclaimprobe`, compose project `reclaim-e2e`, ports 8000/8080). There is no Redis client. Source of the client is crowdsec-bouncer `pkg/simpleredis` @ `6548da47`. Pinned facts: `knowledge/research/ext_crowdsec_simpleredis/`. Traefik GOPATH loader: `knowledge/research/ext_traefik_plugins_local-loader/`. Specs: `std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-commands`. Explore decisions: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Copy crowdsec `pkg/simpleredis` into `simpleredis/` with import-path edits only.
- Prove the client under Yaegi (`go test` interp + Traefik local plugin) without changing reclaim e2e semantics.

**Non-Goals:**
- Crowdsec `pkg/cache`, LAPI, captcha, decisionscope.
- `go-redis`, miniredis, TLS, Unix sockets.
- A product-root LICENSE or a new `go.mod` Redis require.
- Inventing SimpleRedis Language (parked; usage packet after the package exists).
- Renaming compose project `reclaim-e2e`.

## Decisions

1. **Package `simpleredis/` (`package simpleredis`).** Dest sibling `reclaim/` is folder=package. README Layout `redis/` is a Planned placeholder. Alternative: rename the package to `redis` to match README — rejected; that would rename a copied API whose error strings and type `SimpleRedis` are the contract.

2. **Byte-faithful copy @ `6548da47`.** Change the module prefix to this repo. Keep `Init`/`Get`/`MGet`/`Set`/`Del`/`Close`, exported error strings, pool caps, `ioError` `errors.Is(os.ErrDeadlineExceeded)` (no `net.Error` assert). Copy `simpleredis_test.go` (in-process fake TCP). Alternative: rewrite against go-redis — rejected; out of scope and Yaegi would interpret another module.

3. **Nested fake plugin, not root `plugin.go`.** Same reason as reclaim: this repo is a library. Module `github.com/david-garcia-garcia/simpleredisprobe` under `e2e/simpleredisprobe/` exports `Config`, `CreateConfig`, `New`. `New` Inits only (host from Config, default `redis:6379`). `ServeHTTP` SET+GET against compose Redis and sets `X-SimpleRedis-Value` to the GET bytes. Compose adds:
   - `./e2e/simpleredisprobe` → `plugins-local/src/github.com/david-garcia-garcia/simpleredisprobe`
   - existing repo-root mount (already present for reclaim)
   - `--experimental.localplugins.simpleredisprobe.modulename=...` with `useunsafe=false`
   Alternative: a second compose stack on new ports — rejected; Desired 3 follows dest reclaim (one compose, one runner).

4. **Share `reclaim-e2e`.** Keep project name, `reclaim-e2e-traefik`, ports 8000/8080, routes `/a` `/b`, Traefik `v3.7.11`. Add `redis:7-alpine` (hostname `redis`, port 6379, no password, empty database), whoami `whoami-redis` on PathPrefix `/redis` with the new middleware. Pester Redis Describe must not stop `whoami-a`/`whoami-b`. Alternative: fake RESP inside the plugin — rejected; e2e is Traefik+Yaegi+network.

5. **Yaegi `go test` owns the fake listener.** `simpleredis/yaegi_test.go` mirrors `reclaim/yaegi_test.go`: GOPATH copy of non-test sources, `interp` with `stdlib` only. Compiled test listens; interpreted probe calls `Init`/`Get`/`Set`/`Del`. Alternative: only Pester — rejected; dest proves reclaim both in-process and under Traefik.

6. **One integration runner.** Extend `Test-Integration.ps1` with Redis TCP or `/redis` HTTP wait (HTTP `/redis` after Traefik+Redis are up). One Pester file, second Describe. CI failure logs also dump `redis` and `whoami-redis`. Do not split jobs.

7. **Apache-2.0 on the copied package only.** `simpleredis/LICENSE` with the source appendix copyrights (Containous SAS, Traefik Labs). README notes the copied client is Apache-2.0. Do not add a repo-root LICENSE (would relicense reclaim). Debt already notes a later product-license choice.

## Risks / Trade-offs

- [Compose project still named `reclaim-e2e` after Redis joins] → Mitigation: do not rename this change; follow-up is `knowledge/debt/2026-09-11-rename-reclaim-e2e-compose.md`.
- [GOPATH dual plugin mounts fail to resolve simpleredis] → Mitigation: probe imports only this module’s `simpleredis`; no third-party deps; same replace layout as `reclaimprobe`.
- [Redis late at Traefik start] → Mitigation: `New` Inits only; first SET+GET is in `ServeHTTP` after compose wait.
- [Reclaim Pester stops a/b and breaks `/redis`] → Mitigation: Redis Describe never stops `whoami-a`/`whoami-b`; reclaim Describe stays owner of that stop/start.
- [No root LICENSE while Apache-2.0 files land] → Mitigation: package-local LICENSE; product-wide license is `knowledge/debt/2026-09-11-choose-product-license.md`.

## Migration Plan

New library. No production deploy. Rollback is revert the branch. Crowdsec can switch cache imports after this lands; not this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
