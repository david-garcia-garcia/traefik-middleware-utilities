# Requirement
IssueKey: 2026-11-22-tokenbucket-expired-bucket-reclaim

El texto del solicitante es un brief de diseño completo (`ticket/source.md`, spec del llamante con `issueHost: local`). Se copia íntegro a continuación; las secciones posteriores (`Current (code)`, `Affected`, `Out of scope`, `Unknowns`, `Tensions`) son el análisis de prepare y no sustituyen al brief. El **Design** del brief es vinculante: propose lo transcribe, no lo rediseña.

---

## Requirement (TEST ONLY (not reproduced: Memory drops expired buckets whenever a new key arrives; Redis keys carry EXPIRE))
Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13704
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue). READ ONLY. Do not modify that tree.

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)
No product change. Regression tests: ten sources idle past ttl plus one new source leave exactly one bucket in Memory; on live Redis/Dragonfly the bucket key has a PTTL in (0, ttl] after Allow.
Record as a follow-up note (devstate issues + knowledge/debt, not taken): Memory.dropExpired scans the whole map (up to 65536 entries) under the global mutex on every new key, and dropOne evicts an arbitrary live key instead of the one closest to expiry; both need a benchmark (compiled and Yaegi) before any change.

## Tests
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13704_MemoryReclaimsExpiredBuckets and TestTraefik13704_RedisBucketExpires -> tokenbucket/traefik_issue13704_test.go and tokenbucket/traefik_issue13704_e2e_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).
Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

---

## Upstream

La existencia del defecto se tomó de este informe upstream (no hay PR upstream abierto contra ese issue):

- Issue: https://github.com/traefik/traefik/issues/13704

En Traefik, `ttlmap` solo libera una clave caducada si alguien la vuelve a consultar, o cuando el mapa llega a 65 536 entradas y entonces libera solo una; la memoria crece con el número de fuentes distintas, no con las activas. Ese síntoma **no se da** en este producto (C2 del análisis de repros: Memory ya retiene un solo bucket tras diez fuentes ociosas más una nueva; Redis deja PTTL en `(0, ttl]`). Este cambio es **solo tests de regresión** de ese comportamiento correcto. El cuerpo del PR y la tarjeta de entrega deben citar el issue anterior.

## Problem

Este repo no reproduce el fallo de Traefik #13704. Falta una prueba autocontenida que fije: (1) diez fuentes ociosas más allá de `ttl` más una fuente nueva dejan exactamente un bucket en `Memory`; (2) en Redis/Dragonfly reales, tras `Allow`, la clave del bucket tiene `PTTL` en `(0, ttl]`.

## Current (code)

Base: `origin/master` @ `7a435ac246876370b0203890e57534a686592cb6`.

- Memory, clave nueva o caducada: `tokenbucket/memory.go` (`Memory.Allow`) — si la entrada existe y `now` no es anterior a `expireAt`, la borra (`removeLocked`) y la trata como ausente; si sigue ausente, llama `makeRoomLocked` y guarda un bucket lleno.
- Memory, purga al alta: `tokenbucket/memory.go` (`makeRoomLocked`) — recorre la cabeza de la lista de caducidad (`head`/`tail` en `Memory`, `prev`/`next` en `memEntry`) y borra mientras `head` esté caducada; si el mapa sigue en `maxMemorySources` (65536), borra `head` aunque esté viva. No hay `dropExpired` ni `dropOne`.
- Memory, tope y caducadas primero: `openspec/specs/std_go_tokenbucket_allow/spec.md` (requisito "Memory store caps sources and removes expired ones first") y tests en `tokenbucket/memory_test.go` (`TestMemory_FullStoreDropsExpiredBeforeLive`, `TestMemory_FullStoreOfLiveStaysAtCap`, `TestMemory_ReusedSourceMovesBehindExpiring`). Esos tests cubren el tope 65536, no el caso de diez fuentes bajo el tope.
- Memory, diez ociosas + una nueva = un bucket: `not found` — no hay `tokenbucket/traefik_issue13704_test.go` ni `TestTraefik13704_MemoryReclaimsExpiredBuckets` en este árbol. El repro de referencia vive fuera, en el árbol de repros (solo lectura): `tokenbucket/traefik_issues_test.go` (`TestTraefik13704_MemoryReclaimsExpiredBuckets`).
- Redis, caducidad de clave: `tokenbucket/lua.go` (`allowScript`) — cada EVAL hace `redis.call('expire', key, ttl)` tras el `HSET`. `tokenbucket/redis.go` (`Redis.Allow`) pasa `ttlSeconds` como ARGV[3]. Construcción: `tokenbucket/clock.go` (`validateClock`, `ttlSeconds`).
- Redis, PTTL en vivo tras Allow: `not found` — `tokenbucket/limiter_e2e_test.go` (`TestLive_Limiter`) no lee `PTTL`. No hay `tokenbucket/traefik_issue13704_e2e_test.go` ni `TestTraefik13704_RedisBucketExpires`. El repro de referencia vive fuera: `tokenbucket/traefik_issues_e2e_test.go` (`TestTraefik13704_RedisBucketExpires`).
- Gating live: `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`, `TestLive_Limiter`) — skip con `-short` o sin `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`.
- Firma de `Allow` en dest: `tokenbucket/memory.go`, `tokenbucket/redis.go` — `(Status, error)`. Los tests de referencia del árbol de repros llaman `Allow` con tres valores de retorno (API anterior).
- Bancos Memory ya presentes: `tokenbucket/bench_test.go` (`BenchmarkMemoryAllow`) y `tokenbucket/bench_yaegi_test.go` (`BenchmarkYaegiMemoryAllow`), incluidos escenarios de tabla llena / clave nueva.

## Desired

Sin cambio de producto. Tests de regresión:

1. Diez fuentes ociosas más allá de `ttl` más una fuente nueva dejan exactamente un bucket en `Memory`.
2. En Redis y Dragonfly reales, tras `Allow`, la clave del bucket tiene `PTTL` en `(0, ttl]`.
3. Ficheros nuevos autocontenidos: `tokenbucket/traefik_issue13704_test.go` y `tokenbucket/traefik_issue13704_e2e_test.go`. Helpers propios con sufijo `13704`. No ficheros helper compartidos. No copiar tests de otros issues. Afirmar el comportamiento de un limitador correcto (no tautologías). Conservar los casos de control existentes (`tokenbucket/memory_test.go`, `TestLive_Limiter`). Live: mismo gating que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient` ya existentes.
4. El follow-up pedido sobre `dropExpired` / `dropOne` no se toma en este cambio (ver `devstate/issues.md`).

## Affected

- `tokenbucket/traefik_issue13704_test.go` — nuevo (unidad Memory).
- `tokenbucket/traefik_issue13704_e2e_test.go` — nuevo (live Redis/Dragonfly, `PTTL`).
- `tokenbucket/limiter_e2e_test.go` — solo reutilizar `runForEachLiveEngine` / `waitLiveClient`; no es el sitio de los asserts de este issue.
- `tokenbucket/memory.go`, `tokenbucket/lua.go`, `tokenbucket/redis.go` — sin cambio de producto; son el comportamiento que los tests fijan.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` y `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — el contrato vivo ya describe tope/caducadas y `EXPIRE` en segundos enteros; este ticket no pide un requisito de producto nuevo. Propose decide si hace falta un escenario de spec para el caso diez+uno / `PTTL`.

## Out of scope

- Cualquier cambio de producto en `Memory` o `Redis` (incluido reintroducir o reescribir `dropExpired` / `dropOne`).
- Otros issues upstream (14297, 13957, 14173, 13706, 13618, 12091, 12078, 6113, 10700, 14328).
- Helpers compartidos entre PRs paralelos; copiar tests de otros issues.
- Modificar el árbol de repros `traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros`.
- Medir o cambiar `backendbackoff.Gate` (`dropExpired` / `dropOne` siguen ahí; ya hay deuda en `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`).

## Unknowns

- Cómo adaptar los tests de referencia a `(Status, error)` sin copiar helpers de otros issues del mismo fichero combinado (explore/implement: el cuerpo de #13704 no usa `countAdmits`; el e2e sí usa `liveKey` / `evalKey` locales de ese árbol).
- Si propose añade un escenario al spec vivo para el caso diez+uno / `PTTL` o deja el contrato como está (el Design dice "No product change").
- Comportamiento de `PTTL` en Dragonfly frente a Redis en el motor local (el análisis de repros midió PTTL 6999 ms en Redis con `ttl` 7 s; no se re-ejecuta aquí).

## Tensions

- El Design pide anotar `Memory.dropExpired` (recorrido O(n) del mapa bajo el mutex) y `dropOne` (víctima arbitraria) como follow-up no tomado, con bancos compilado y Yaegi antes de cualquier cambio. En dest esas funciones **no existen**: las sustituyó `makeRoomLocked` (lista de caducidad) en el cambio `2026-11-21-ratelimit-benchmarks-key-scaling`, y ya hay `BenchmarkMemoryAllow` / `BenchmarkYaegiMemoryAllow`. `devstate/issues.md` marca esa fila `[x]` Taken en dest; no se abre un fichero de deuda nuevo que nombre APIs ausentes.
- El brief dice TEST ONLY / "not reproduced" y a la vez obliga a citar el issue upstream como origen del defecto. El defecto es de Traefik; aquí se fija el comportamiento correcto, no se arregla un fallo local.
- Los tests de referencia llaman `Allow` con tres retornos; dest solo tiene dos. Implement debe adaptar, no copiar el fichero combinado.
- `TestMemory_FullStoreDropsExpiredBeforeLive` ya cubre caducadas-antes-que-vivas en el tope 65536; el ticket igual exige el caso diez+uno bajo el tope en un fichero propio.
