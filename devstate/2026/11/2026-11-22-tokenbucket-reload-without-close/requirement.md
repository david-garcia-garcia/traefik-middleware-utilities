# Requirement
IssueKey: 2026-11-22-tokenbucket-reload-without-close

El texto del solicitante es un brief de diseño completo (`ticket/source.md`, spec del llamante con `issueHost: local`). Se copia íntegro a continuación; las secciones posteriores (`Problem`, `Current (code)`, `Desired`, `Affected`, `Out of scope`, `Unknowns`, `Tensions`) son el análisis de prepare y no sustituyen al brief. El diseño del brief es vinculante: propose lo transcribe, no lo rediseña.

---

## Requirement (TEST ONLY (not reproduced: simpleredis starts no background goroutines; dropped clients release their sockets on GC))
Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13618 ; upstream PR https://github.com/traefik/traefik/pull/13619 (open)
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)
No product change. Regression test simulating 50 reloads: each builds a new simpleredis client + Redis limiter through a counting TCP proxy, runs one Allow and drops it without Close. After GC no proxied socket stays open and no goroutine survives (excluding the proxy's own). Live Redis and Dragonfly. Keep only the proxy features this test needs (no write hold).

## Tests
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13618_ReloadWithoutClose (uses the counting TCP proxy) -> tokenbucket/traefik_issue13618_e2e_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).
Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

---

## Upstream

La existencia del defecto se tomó de estos informes de Traefik (no de un fallo medido en este repo):

- Issue https://github.com/traefik/traefik/issues/13618
- PR upstream https://github.com/traefik/traefik/pull/13619 (abierto)

En Traefik cada recarga crea un `redis.NewUniversalClient` y no lo cierra; desde go-redis v9.12 cada cliente arranca `CircuitBreakerManager.cleanupLoop` y el proceso acumula goroutines y memoria. El análisis C3 de `TRAEFIK-UPSTREAM-ISSUES.md` y el repro `TestTraefik13618_ReloadWithoutClose` (en el worktree de referencia, no en dest) concluyen que **aquí no se da**: `simpleredis` no arranca goroutines de fondo y, tras GC, los sockets de clientes abandonados no quedan abiertos.

## Problem

Traefik #13618 es una fuga de clientes go-redis por recarga. Este repo no usa go-redis. Falta un test de regresión en dest que fije el comportamiento correcto: 50 recargas que construyen cliente + `NewRedis`, un `Allow`, y se descartan sin `Close`, no deben dejar sockets proxied ni goroutines (salvo las del proxy).

## Current (code)

Base: `origin/master` @ `7a435ac`.

- `New` no marca ni arranca goroutines; copia `Config`, construye el semáforo in-use y no marca: `simpleredis/simpleredis.go` (`func New`). Contrato: `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (`New` MUST NOT start a goroutine to close idle sockets).
- En el paquete de producción `simpleredis/*.go` no hay `go func` (solo tests). No hay `runtime.SetFinalizer` en el árbol.
- `Close` drena ociosos y bloquea nuevos dials: `simpleredis/simpleredis.go` (`func (sr *SimpleRedis) Close`). Contrato: `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (Close drains the pool).
- Tras un comando limpio el socket vuelve a `idleConns`; sin `Close` permanece allí hasta que alguien lo drena o el cliente deja de ser alcanzable: `simpleredis/pool.go` (`release`, `parkIdleConn`).
- El barrido de `IdleTimeout` ocurre en el siguiente `borrow`, no en un reaper: `simpleredis/pool.go` (`takeIdleConn`). Un cliente sin más comandos puede conservar ociosos más allá de `IdleTimeout` (mismo spec).
- El limitador Redis guarda el `*simpleredis.SimpleRedis` y no tiene `Close`; `Allow` es `(Status, error)`: `tokenbucket/redis.go` (`type Redis`, `NewRedis`, `Allow`). Contrato: `openspec/specs/std_go_tokenbucket_allow/spec.md`.
- `reclaim/` conserva un valor entre recargas; este ticket no pide usarlo ni cambiarlo: `reclaim/table.go`.
- Gate live de tokenbucket: `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`, `TestLive_Limiter`). Env `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`. Skip con `-short` o sin esas vars. CI: `openspec/specs/std_go_ci_test-suites/spec.md`.
- Gate live de simpleredis (no es el de este test): `simpleredis/simpleredis_e2e_test.go` (`runForEachLiveEngine`), `simpleredis/newtest_test.go` (`newTestRedis`).
- `TestTraefik13618_ReloadWithoutClose` — not found en dest.
- Proxy TCP que cuenta sockets abiertos — not found en dest.
- El repro de referencia (solo lectura, otro worktree) vive en `tokenbucket/traefik_issues_e2e_test.go` (`TestTraefik13618_ReloadWithoutClose`, `startCountingProxy`, `countingProxy`); no en `tokenbucket/traefik_issue13618_e2e_test.go`. Allí `Allow` se llama con tres valores; dest ya es `(Status, error)`. El proxy de referencia tiene `holdWrites` (retención de escrituras para #14173); este ticket pide no copiar esa capacidad.

## Desired

Sin cambio de producto. Un test live, autocontenido para #13618, que simule 50 recargas: cada una `simpleredis.New` + `NewRedis` contra un proxy TCP que cuenta sockets (sin retención de escrituras), un `Allow`, y se descarta sin `Close`. Tras GC: `proxy.open == 0` y no sobreviven goroutines salvo las del proxy. Redis y Dragonfly. Helpers propios con sufijo `13618`. Gate igual que `TestLive_Limiter`. Afirmar el comportamiento de un limitador correcto (no tautologías). Conservar casos de control si el repro los tiene.

## Affected

- `tokenbucket/` — fichero(s) de test nuevos para #13618 (p. ej. `tokenbucket/traefik_issue13618_e2e_test.go` y helpers con sufijo `13618` en el mismo paquete).
- Reutiliza `runForEachLiveEngine` / `waitLiveClient` de `tokenbucket/limiter_e2e_test.go`.
- CI `e2e-redis` y `e2e-dragonfly` ejecutarán el test; `Unit` / `Unit race` lo saltan por `-short`.
- Producto (`simpleredis`, `tokenbucket` runtime, `reclaim`, Lua) — no tocado.

## Out of scope

- Arreglar o reimplementar el cliente Redis de Traefik / go-redis.
- Compartir el cliente con `reclaim/` en el middleware consumidor.
- Añadir `Close` al limitador Redis.
- Product change en `simpleredis` (reaper, finalizers, Close implícito).
- Copiar tests u helpers de otros issues (#14297, #13957, #14173, retención de escrituras del proxy).
- Ficheros helper compartidos entre PRs paralelos.
- Análisis de issues upstream distintos de #13618.

## Unknowns

- Si dest HEAD sigue pasando la misma aserción live que el repro de referencia (el repro pasó el 2026-11-22 contra Redis y Dragonfly; dest no tiene el test).
- El cierre del fd al GC es del `netFD` de la stdlib, no de este paquete; el test mide el síntoma (sockets proxied a 0), no el mecanismo.
- Umbral de goroutines residuales del repro (`> 2` falla): si dest lo mantiene o lo ajusta lo decide implement al transcribir el repro.

## Tensions

- El brief nombra `tokenbucket/traefik_issue13618_e2e_test.go`; el repro de referencia está en `tokenbucket/traefik_issues_e2e_test.go` mezclado con otros issues. Desired: fichero propio de #13618, no copiar el archivo compartido.
- El repro llama `Allow` con tres valores; dest es `(Status, error)` (`tokenbucket/redis.go`). Transcribir, no restaurar la firma antigua.
- El proxy de referencia tiene `holdWrites` para #14173; el diseño pide no conservarlo.
- La regla general pide conservar casos de control. Este issue **no se reproduce**; el repro no tiene un control invertido (la fuga de Traefik no ocurre). El test afirma el comportamiento correcto. No inventar un control que fuerce un fallo.
- "TEST ONLY (not reproduced)" y "no product change" coinciden: no hay tensión de producto. No rediseñar.
