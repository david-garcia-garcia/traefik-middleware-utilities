# Requirement
IssueKey: 2026-11-22-tokenbucket-concurrent-first-requests

## Upstream

La existencia del defecto se toma de los informes upstream, no de un hallazgo local inventado:

- Issue: https://github.com/traefik/traefik/issues/14328
- Primera mención: https://github.com/traefik/traefik/issues/13957
- Pull request upstream: ninguno. El issue #14328 sigue abierto (`kind/bug/confirmed`). La sección C1 de `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md` no cita un PR.

## Problem

Traefik confirma una carrera en las primeras peticiones concurrentes de una fuente nueva: `Get` y `Set` del `ttlmap` toman el lock por separado, cada goroutine crea un bucket lleno y la última escritura gana. En el repro upstream, 18 de 20 rondas superan `burst = 5` (hasta 13 de 16 admitidas). Este repo pide **solo tests de regresión** que demuestren que ese defecto no ocurre aquí: 16 primeras peticiones simultáneas de una fuente nueva nunca superan burst (1 req/min, burst 5, maxDelay 500 ms). Sin cambio de producto.

## Current (code)

- `tokenbucket/memory.go` `Memory.Allow`: toma `m.mu` (`Lock`/`Unlock` diferido) y lo mantiene durante la búsqueda en `buckets`, el borrado por TTL, `makeRoomLocked`/`storeLocked` si la clave falta, `consumeOne` y la escritura de `expireAt`. No hay ventana entre lookup, create y consume.
- `tokenbucket/redis.go` `Redis.Allow`: un `Eval` de `allowScript` (`allowScriptDigest`). Go no hace GET/SET del hash; el script es dueño de `HGETALL`/`HSET`/`EXPIRE` (`tokenbucket/lua.go`).
- `openspec/specs/std_go_tokenbucket_allow/spec.md`: el almacén en proceso lee `now` con el mutex de `buckets` ya tomado. No hay escenario que cuente 16 primeras peticiones simultáneas contra burst.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`: los campos del hash cambian solo dentro del script Eval.
- `tokenbucket/status.go` `Status.Allowed`: verdadero cuando la espera en microsegundos es ≤ `maxDelay` truncado. Con 1 req/min, burst 5 y maxDelay 500 ms, las admite más allá de burst tienen espera de minutos y `Allowed` es falso.
- `tokenbucket/limiter_e2e_test.go` `liveEngineAddrs` / `runForEachLiveEngine` / `TestLive_Limiter`: skip con `-short` o sin `TOKENBUCKET_LIVE_REDIS` y `TOKENBUCKET_LIVE_DRAGONFLY`. `waitLiveClient` abre `simpleredis.New` con `Config{Host: addr}` (PoolSize por defecto) y reintenta Eval.
- `tokenbucket/*_test.go`: no hay test concurrente de primeras peticiones (no hay `WaitGroup` ni barrera en el paquete).
- `tokenbucket/traefik_issue14328_test.go` — not found
- `tokenbucket/traefik_issue14328_e2e_test.go` — not found
- `tokenbucket/traefik_issues_test.go` — not found en dest (el repro de referencia vive fuera: `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\traefik_issues_test.go` `TestTraefik14328_MemoryConcurrentFirstRequests` y `traefik_issues_e2e_test.go` `TestTraefik14328_RedisConcurrentFirstRequests`).

## Desired

- Sin cambio de producto. No tocar `Memory.Allow`, `Redis.Allow` ni `allowScript`.
- Añadir `tokenbucket/traefik_issue14328_test.go` con `TestTraefik14328_MemoryConcurrentFirstRequests`: 20 rondas, 16 goroutines en barrera compartida, 1 req/min, burst 5, maxDelay 500 ms; ninguna ronda admite más de burst.
- Añadir `tokenbucket/traefik_issue14328_e2e_test.go` con `TestTraefik14328_RedisConcurrentFirstRequests`: 10 rondas, PoolSize 16, misma carga, contra Redis y Dragonfly vivos. Misma puerta que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient`.
- Archivos autónomos de este issue. Helpers en el propio archivo con sufijo `14328` (p. ej. `countAdmits14328`, `liveKey14328`). No crear helpers compartidos ni copiar tests de otros issues.
- Afirmar el comportamiento de un limitador correcto (admits ≤ burst), no tautologías. Conservar los casos de control si el repro de referencia los tiene.
- El cuerpo del PR dice que el defecto upstream no ocurre aquí y por qué (mutex único en Memory; script atómico en Redis) y nombra https://github.com/traefik/traefik/issues/14328 y https://github.com/traefik/traefik/issues/13957.
- Compilación contra el `Allow` de dest: `(Status, error)`; contar `Status.Allowed`.
- Verde local: `go vet ./...`; `go test -short -count=1 ./...`; tests vivos del paquete con `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` y `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380`. CI: golangci-lint v1.63.4 (unused/unparam/godot/thelper), Unit, Unit race, e2e-redis, e2e-dragonfly.

## Affected

- `tokenbucket/traefik_issue14328_test.go` (nuevo)
- `tokenbucket/traefik_issue14328_e2e_test.go` (nuevo)
- Reutiliza `tokenbucket/limiter_e2e_test.go` `runForEachLiveEngine` / `waitLiveClient` (sin extraer helpers nuevos)

## Out of scope

- Arreglar Traefik o fusionar un PR upstream (no hay PR de #14328).
- Cambiar `tokenbucket/memory.go`, `tokenbucket/redis.go` o `tokenbucket/lua.go`.
- Tests de otros issues (#13957 TTL, #13704, HMGET, etc.).
- Helpers compartidos entre PRs paralelos.
- `simpleredis` salvo `PoolSize` en el cliente del test vivo de este issue.
- Documentar o medir `dropExpired` bajo el mutex (C2 del análisis; no es este ticket).

## Unknowns

- Si el par de referencia 14328 incluye un caso de control aparte de «admits ≤ burst». En el worktree de repro, Memory son 20 rondas y Redis 10; no hay subtest de control tipo TTL.
- Cómo adaptar `waitLiveClient` (PoolSize por defecto 8) al PoolSize 16 que pide el diseño para que las 16 peticiones se solapen en conexiones distintas. Prepare no lo resuelve.
- Los números del repro en **este** worktree. El análisis C1 dice que aquí pasa; no se ha ejecutado el test en dest.
- El `Allow` del repro de referencia desempaqueta tres valores; dest devuelve `(Status, error)`. El test nuevo debe usar `Status.Allowed`; no se ha compilado el repro contra dest.

## Tensions

- El diseño pide PoolSize 16 en el test vivo. Las reglas de tests mandan usar `waitLiveClient`, que construye `simpleredis.New(Config{Host: addr})` sin PoolSize. Las dos instrucciones no caben en una sola llamada a ese helper.
- TEST ONLY y «el defecto no se da aquí»: el CI debe quedar verde sin cambio de producto. Si dest admitiera más de burst, el diseño no autoriza un arreglo; eso sería otro ticket.
