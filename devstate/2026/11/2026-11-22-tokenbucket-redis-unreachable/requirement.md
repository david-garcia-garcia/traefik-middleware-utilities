# Requirement
IssueKey: 2026-11-22-tokenbucket-redis-unreachable

Fuente: pedido del humano en chat (2026-11-22), `ticket/source.md`. Base: `origin/master` @ `7a435ac2`. TEST + DOC; el defecto de política no se reproduce en esta librería.

## Upstream

The existence of the defect was taken from these upstream reports:

- https://github.com/traefik/traefik/issues/12091 (closed upstream as duplicate of https://github.com/traefik/traefik/issues/12043 on 2026-04-16; no upstream PR closed #12091)

## Problem

En Traefik, si Redis no responde, el middleware `rateLimit` contesta HTTP 500 `Could not insert/update bucket` y no hay opción de fail-open ni fail-closed ([traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091)). En este repo la librería ya devuelve el error de `Eval` y deja la política al llamador. El pedido es TEST + DOC: un test autocontenido que fije `errors.Is(err, simpleredis.ErrUnreachable)` y `allowed=false` contra un puerto loopback que rechaza, y GoDoc en `Redis.Allow` que nombre `ErrUnreachable` / `ErrTimeout` y que fail-open o fail-closed es decisión del middleware. No hay cambio de comportamiento.

## Current (code)

- `Redis.Allow` GoDoc: consume vía Eval y «The Status is the zero value when the error is non-nil». No nombra `simpleredis.ErrUnreachable`, `ErrTimeout`, ni que fail-open/fail-closed lo decide el middleware — `tokenbucket/redis.go:52-53`.
- `Redis.Allow` ante error de `Eval` devuelve `Status{}, err` (Status cero ⇒ `Allowed=false`) — `tokenbucket/redis.go:67-69`.
- Contrato vivo: error no nil ⇒ Status cero y `Allowed` false (contexto cancelado, error Redis, o Eval malformado) — `openspec/specs/std_go_tokenbucket_allow/spec.md` (Scenario: Error returns the zero status).
- `Eval` propaga el error de `exec` — `simpleredis/commands_eval.go:28-35`. Un fallo de I/O que no es deadline ni rechazo TLS es `errUnreachable`; deadline es `errTimeout` — `simpleredis/resp.go:394-404` (`ioError`).
- Centinelas exportados `ErrUnreachable` y `ErrTimeout` — `simpleredis/simpleredis.go:28-31`.
- `TestRedis_Unreachable` apunta a `127.0.0.1:1` y compara `err.Error() == simpleredis.RedisUnreachable`; no usa `errors.Is` y no lee `Allowed` — `tokenbucket/limiter_test.go:230-239`. Helper `newSimpleRedisForTest` — `tokenbucket/limiter_test.go:16-23`.
- `TestStatus_ErrorReturnsTheZeroStatus` en `127.0.0.1:1` exige `err.Error() == simpleredis.RedisUnreachable` y `got == (Status{})` — `tokenbucket/status_test.go:164-181`.
- Fichero dedicado `tokenbucket/traefik_issue12091_test.go` — `not found`.
- `TestTraefik12091_RedisUnreachable` — `not found`.
- El usage doc pide casar errores Redis por texto de `Error()` — `knowledge/devdocs/std_go_tokenbucket.md:28`.
- Gating live: `runForEachLiveEngine` / `waitLiveClient` / `TestLive_Limiter` saltan con `-short` o sin `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY` — `tokenbucket/limiter_e2e_test.go:73-86`, `209`.
- Opción fail-open / fail-closed en `tokenbucket` — `not found` (la librería no elige política HTTP).

## Desired

Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign):

No behavior change. Test: Redis limiter pointed at a refusing loopback port returns errors.Is(err, simpleredis.ErrUnreachable) and allowed=false. GoDoc on Redis.Allow: on an outage it returns simpleredis.ErrUnreachable (or ErrTimeout) with allowed=false; fail-open or fail-closed is the middleware's decision (upstream Traefik answers 500 with no option, traefik/traefik#12091).

Tests (binding):

- Mapear `TestTraefik12091_RedisUnreachable` (`traefik_issues_test.go` en el árbol de referencia) → `tokenbucket/traefik_issue12091_test.go`.
- El fichero de test es autocontenido para este issue. Los helpers que necesite viven en ese fichero, con nombres sufijados por el número de issue (p. ej. `countAdmits12091`, `liveKey12091`); no crear ficheros de helpers compartidos y no copiar tests de otros issues.
- Los tests afirman el comportamiento que debe tener un limitador correcto (sin tautologías); se conservan los casos de control.
- Si hay tests live, el mismo gating que `TestLive_Limiter` (skip con `-short` o sin las env) vía `runForEachLiveEngine` / `waitLiveClient` en `tokenbucket/limiter_e2e_test.go`.
- Motores locales: `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` `SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381` `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382` (password `secret`).
- Puerta local antes de push: `go vet ./...` ; `go test -short -count=1 ./...` ; los tests live del paquete tocado con esas env. CI GitHub (golangci-lint v1.63.4 con unused/unparam/godot/thelper, Unit, Unit race, e2e-redis, e2e-dragonfly) en verde.

## Affected

- `tokenbucket/redis.go` (GoDoc de `Allow`; el cuerpo no debe cambiar de comportamiento).
- `tokenbucket/traefik_issue12091_test.go` (nuevo).
- Contrato vivo `openspec/specs/std_go_tokenbucket_allow/spec.md` solo si propose añade un escenario que nombre los centinelas (hoy el escenario de error es genérico).
- Usage `knowledge/devdocs/std_go_tokenbucket.md` solo si explore/propose resuelven la tensión del casado por `Error()`; el pedido nombra GoDoc de `Redis.Allow`.

## Out of scope

- Cambiar el comportamiento de `Redis.Allow` (el Design lo prohíbe).
- Añadir política fail-open / fail-closed o un código HTTP en la librería (lo decide el middleware).
- Reproducir el 500 de Traefik.
- Copiar tests de otros issues o crear helpers compartidos.
- Cambiar los centinelas de `simpleredis`.
- Implementar el issue duplicado [traefik/traefik#12043](https://github.com/traefik/traefik/issues/12043).
- Editar el árbol de referencia `tokenbucket-traefik-upstream-repros`.

## Unknowns

- Si basta el GoDoc de `Redis.Allow` o hay que alinear `knowledge/devdocs/std_go_tokenbucket.md:28` («Match Redis errors by `Error()` text») con `errors.Is`.
- Si el spec vivo necesita un escenario que nombre `ErrUnreachable` / `ErrTimeout` (hoy dice «Redis error»).
- Si hace falta un test del camino `ErrTimeout` (el Design lo nombra en GoDoc; el test nombrado es solo puerto que rechaza).
- Si este issue necesita un test live: el repro de referencia es unitario contra `127.0.0.1:1`, no contra los motores Docker.
- Cómo adaptar la firma del repro de referencia (`Allow` devolvía `bool, wait, error` en ese árbol) a `(Status, error)` de dest (`#106`).

## Tensions

- Pedido TEST + DOC y «not reproduced» vs código: `Redis.Allow` ya propaga el error de Eval y el Status cero. Los tests existentes casan por `Error()` (`tokenbucket/limiter_test.go:230-239`, `tokenbucket/status_test.go:164-181`). El hueco es `errors.Is` en un fichero dedicado y el GoDoc de centinelas, no un return path ausente.
- Usage `knowledge/devdocs/std_go_tokenbucket.md:28` pide casar por texto; el Design pide `errors.Is`. El pedido nombra solo GoDoc de `Redis.Allow`.
- El Design nombra `ErrTimeout` en GoDoc; la sección Tests solo nombra `TestTraefik12091_RedisUnreachable` (puerto que rechaza).
- «Keep the control cases» vs el test de referencia, que no trae un control live; `TestLive_Limiter` ya cubre el camino feliz en `tokenbucket/limiter_e2e_test.go`.
