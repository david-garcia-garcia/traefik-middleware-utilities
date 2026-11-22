# Requirement
IssueKey: 2026-11-22-tokenbucket-memory-instance-scope

El texto del solicitante es un brief de diseño completo (`ticket/source.md`, spec del llamante con `issueHost: local`). Se copia íntegro a continuación; las secciones posteriores (`Upstream`, `Current (code)`, `Desired`, `Affected`, `Out of scope`, `Unknowns`, `Tensions`) son el análisis de prepare y no sustituyen al brief. El diseño del brief es vinculante: propose lo transcribe, no lo rediseña.

---

## Requirement (DOCUMENT + TEST (reproduced as a contract difference; not fixable inside the library))
Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13706 ; upstream PRs https://github.com/traefik/traefik/pull/13823 (open, scope option) and https://github.com/traefik/traefik/pull/13707 (closed)
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue). You may READ that file. Do not modify that worktree.

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)
Upstream: Traefik builds one in-memory limiter per router, so N routers give each source N budgets, while Redis keys by middleware and shares one budget. In this library each *Memory is an independent key space and Redis limiters on the same key share state; which one a middleware gets depends on how it builds and shares limiters, not on the library.
Binding design:
- No behavior change. Make the scope part of the documented contract: GoDoc on Memory/NewMemory (each value is its own key space; to share a budget across routers or reloads, share one *Memory, e.g. via reclaim keyed by middleware name) and on Redis/NewRedis (state lives in the server; any limiter on the same key shares it, so the key prefix defines the scope).
- Rewrite the repro test so it asserts the contract instead of failing: (a) two routers sharing one *Memory admit burst in total; (b) two separate *Memory values admit 2*burst (documented per-instance behavior, with a comment citing traefik/traefik#13706); (c) live: two Redis limiters on one key admit burst in total. A middleware-level fix belongs to the consumer and is out of scope; record it as a note in devstate issues + knowledge/debt.

## Tests
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13706_MemoryInstancesPerRouter (traefik_issues_test.go) and TestTraefik13706_RedisInstancesShare (traefik_issues_e2e_test.go) -> tokenbucket/traefik_issue13706_test.go and tokenbucket/traefik_issue13706_e2e_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis). Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

---

## Upstream

La existencia del defecto (ámbito distinto entre el limitador en memoria y el de Redis cuando N routers referencian el mismo middleware) se tomó de estos informes de Traefik; este ticket no lo redescubre. Citarlos en el resumen del PR y en la tarjeta de entrega:

- Issue: https://github.com/traefik/traefik/issues/13706
- PR abierto (opción `scope: router | middleware`): https://github.com/traefik/traefik/pull/13823
- PR cerrado: https://github.com/traefik/traefik/pull/13707

Lectura de apoyo (worktree de referencia, solo lectura): `tokenbucket/TRAEFIK-UPSTREAM-ISSUES.md` sección B2 (#13706) en `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros`. Traefik construye un limitador in-memory por router (`BuildMiddlewareChain`) y en Redis usa `rate:<middleware>:<source>`, de modo que N routers dan N presupuestos en memoria y uno en Redis. El PR #13823 añade `scope` y deja el comportamiento actual como valor por defecto de cada backend.

## Current (code)

Base: `origin/master` @ `7a435ac`.

- Cada `*Memory` tiene su propio mapa de buckets; `NewMemory` asigna un `map` nuevo. No hay tabla de proceso ni clave de middleware: `tokenbucket/memory.go` (`type Memory`, campo `buckets`; `func NewMemory`).
- GoDoc de `Memory` / `NewMemory` no dice que cada valor sea un espacio de claves ni cómo compartir presupuesto (routers / recargas): `tokenbucket/memory.go` (`// Memory is an in-process Traefik token bucket…`, `// NewMemory builds an in-process limiter…`).
- `Redis.Allow` hace `Eval` sobre la `key` opaca del llamante; el estado vive en el servidor. Dos `*Redis` sobre la misma clave comparten burst: `tokenbucket/redis.go` (`func (r *Redis) Allow`, `r.redis.Eval(..., []string{key}, args)`).
- GoDoc de `Redis` / `NewRedis` no dice que el prefijo de clave defina el ámbito ni que cualquier limitador sobre la misma clave lo comparta: `tokenbucket/redis.go` (`// Redis is a Traefik token bucket stored as a Redis hash via Eval.`, `// NewRedis builds a Redis limiter on a SimpleRedis from simpleredis.New…`).
- El devdoc ya afirma el share de Redis y que el prefijo es del llamante, no el aislamiento por instancia de `*Memory`: `knowledge/devdocs/std_go_tokenbucket.md` («Two limiter instances on one Redis key share burst»; «Prefix keys in the caller.»; «`NewMemory(...)` or `NewRedis(...)` once.»).
- El spec vivo exige que el prefijo de `key` sea del llamante; no exige ni prohíbe compartir un `*Memory` entre routers: `openspec/specs/std_go_tokenbucket_allow/spec.md` (`Requirement: Allow maps admit, wait, and deny`, «Prefixing of `key` is the caller's job.»).
- Test en vivo que ya cubre (c) sobre Redis/Dragonfly: `tokenbucket/limiter_e2e_test.go` (`TestLive_Limiter` → `twoInstancesShare`). Gate: `runForEachLiveEngine` / `waitLiveClient` / `liveEngineAddrs` (skip con `-short` o sin `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`).
- Tests de contrato #13706 en este árbol: `not found` (`tokenbucket/traefik_issue13706_test.go`, `tokenbucket/traefik_issue13706_e2e_test.go`).
- Helper `liveKey` / `countAdmits` en este árbol: `not found`. El e2e actual usa `t.Name()` como clave: `tokenbucket/limiter_e2e_test.go`.
- `reclaim.Table` guarda un valor por clave (create / sleep / wake / close) y sirve para reutilizar un `*Memory` entre recargas si el consumidor lo clavea por nombre de middleware: `reclaim/table.go` (`type Table`, comentario de estados Open/Sleep/Wake/Close).
- Repro de referencia (otro worktree, no está en dest): `TestTraefik13706_MemoryInstancesPerRouter` espera `total == burst` con dos `NewMemory` y **falla** (admite `2*burst`); `TestTraefik13706_RedisInstancesShare` espera `total == burst` con dos `NewRedis` sobre la misma clave y **pasa**. API de esos tests: `Allow` devolvía `(bool, wait, error)`; en dest `Allow` devuelve `(Status, error)`: `tokenbucket/memory.go`, `tokenbucket/status.go`.

## Desired

Sin cambio de comportamiento del limitador. El ámbito pasa a ser contrato documentado y tests que lo afirman:

- GoDoc `Memory` / `NewMemory`: cada valor es su propio espacio de claves; para compartir presupuesto entre routers o recargas, compartir un `*Memory` (p. ej. `reclaim` claveado por nombre de middleware).
- GoDoc `Redis` / `NewRedis`: el estado vive en el servidor; cualquier limitador sobre la misma clave lo comparte; el prefijo de clave define el ámbito.
- Tests autocontenidos (helpers con sufijo `13706`, sin ficheros compartidos ni copiar tests de otros issues):
  - (a) dos «routers» que comparten un `*Memory` admiten `burst` en total;
  - (b) dos `*Memory` distintos admiten `2*burst` (comentario que cita `traefik/traefik#13706`);
  - (c) en vivo, dos `*Redis` sobre una clave admiten `burst` en total, con el mismo gate que `TestLive_Limiter` (`runForEachLiveEngine` / `waitLiveClient` en `tokenbucket/limiter_e2e_test.go`).
- Destino de ficheros: `tokenbucket/traefik_issue13706_test.go` y `tokenbucket/traefik_issue13706_e2e_test.go`.
- Arreglo a nivel middleware (un limitador por nombre de middleware, al estilo Traefik #13823): nota en `devstate/issues.md` + `knowledge/debt/`, no en el código del limitador.

## Affected

- GoDoc: `tokenbucket/memory.go`, `tokenbucket/redis.go`.
- Tests nuevos: `tokenbucket/traefik_issue13706_test.go`, `tokenbucket/traefik_issue13706_e2e_test.go` (aún `not found`).
- Helpers e2e existentes (solo uso, no mover): `tokenbucket/limiter_e2e_test.go`.
- Devdoc / spec (explore/propose deciden si el GoDoc basta o hay delta): `knowledge/devdocs/std_go_tokenbucket.md`, `openspec/specs/std_go_tokenbucket_allow/spec.md`.
- Nota de deuda: `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md`, índice `devstate/issues.md`.
- `reclaim/` no cambia (solo se cita como medio para compartir un `*Memory`).

## Out of scope

Pedido por el brief:

- Cambiar el comportamiento de `Memory.Allow` / `Redis.Allow` / `NewMemory` / `NewRedis`.
- Arreglo en el middleware consumidor (un `*Memory` por nombre de middleware, o un `scope` tipo Traefik #13823). Queda como nota, no como código de esta librería.

Inferido por prepare, no pedido (se listan, no se toman):

- Unificar el mapa de `*Memory` a nivel de proceso.
- Prefijar `rate:` u otra clave de middleware dentro de `tokenbucket` (el spec y el devdoc lo prohíben: el prefijo es del llamante).
- Copiar tests de otros issues, crear helpers compartidos, o reutilizar identificadores sin sufijo `13706`.
- Cambiar Traefik upstream (#13706 / #13823 / #13707).
- Tocar `simpleredis`, `windowcounter` o `backendbackoff`.
- Introducir un constructor «unlimited» o un store global.

## Unknowns

(Explore los resuelve o los escala; no son rediseño del brief.)

- Si el middleware consumidor de esta librería ya comparte un `*Memory` vía `reclaim` claveado por nombre de middleware (no medido en este árbol; el análisis B2 lo deja explícito).
- Si el contrato vivo necesita un requisito nuevo de ámbito (Memory por instancia, Redis por clave) o basta el GoDoc + tests, con el spec actual («Prefixing of `key` is the caller's job»).
- Redacción exacta del GoDoc (el brief fija el contenido, no las frases).
- Cómo nombrar y acotar el helper de clave en vivo (`liveKey13706`) para no chocar con `t.Name()` de `TestLive_Limiter` ni con PRs paralelos.
- Si `(a)` y `(b)` viven en el mismo test o en subtests; el brief nombra un test de referencia que hoy falla y pide reescribirlo para afirmar el contrato.

## Tensions

- Upstream (#13706) trata el desacuerdo memoria-por-router vs Redis-por-middleware como bug de Traefik; este brief lo trata como contrato de la librería (no arreglable aquí) y pide tests que pasen, no un repro que falle.
- El repro de referencia `TestTraefik13706_MemoryInstancesPerRouter` espera `total == burst` con dos `NewMemory` (falla con 6 vs 3). El diseño vinculante pide (b) `2*burst` en instancias separadas y (a) `burst` solo cuando se comparte un `*Memory`.
- `knowledge/devdocs/std_go_tokenbucket.md` ya documenta el share de Redis; el brief pide el mismo hecho en GoDoc de `NewRedis` y el hecho simétrico (espacio por instancia) en GoDoc de `NewMemory`, que hoy no está.
- `TestLive_Limiter` / `twoInstancesShare` ya cubre (c) en `tokenbucket/limiter_e2e_test.go`; el brief exige además un fichero e2e autocontenido `traefik_issue13706_e2e_test.go` (no borrar el control existente; no extraer helpers compartidos).
- La API de `Allow` en dest es `(Status, error)`, no el triple `(bool, wait, error)` del repro de referencia: los tests nuevos deben usar `Status` (`tokenbucket/status.go`).
- El brief pide registrar el arreglo de middleware como nota `knowledge/debt/` en este cambio (producto versionado) aunque el arreglo en sí está fuera de alcance.
