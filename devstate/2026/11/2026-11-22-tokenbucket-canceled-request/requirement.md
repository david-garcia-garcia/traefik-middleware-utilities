# Requirement
IssueKey: 2026-11-22-tokenbucket-canceled-request

## Upstream

La existencia del defecto se toma de estos informes upstream, no de un hallazgo local independiente:

- Issue: https://github.com/traefik/traefik/issues/14173
- Issue: https://github.com/traefik/traefik/issues/13879
- PR upstream (abierto): https://github.com/traefik/traefik/pull/13880
- PR upstream (cerrado como duplicado): https://github.com/traefik/traefik/pull/14236

Análisis de este issue: sección B1 de `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md`.

## Requirement (TEST + DOC (not reproduced in the library; one side effect found and documented))
Upstream source of this bug (MUST be cited): Issues https://github.com/traefik/traefik/issues/14173 and https://github.com/traefik/traefik/issues/13879 ; upstream PRs https://github.com/traefik/traefik/pull/13880 (open) and https://github.com/traefik/traefik/pull/14236 (closed as duplicate)
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)
Upstream answers 500 when the client is gone because its select cannot tell cancellation apart. Here Allow returns context.Canceled (errors.Is) with allowed=false; a pre-canceled ctx charges nothing in Memory or Redis.
Side effect proven on live Redis: if the request is canceled while the EVALSHA is already on the wire, Allow returns Canceled promptly but the server still runs the script and charges one token (Dragonfly did not run it).
Binding design:
- No behavior change. GoDoc on Redis.Allow: a context.Canceled / DeadlineExceeded return does not prove the script did not run; the token may have been charged. GoDoc on Memory.Allow: a ctx already done returns its error and charges nothing. Mention that the HTTP mapping (499 on Canceled, no wait when wait==0) belongs to the middleware, citing traefik/traefik#14173.
- Tests: Memory pre-canceled (asserts Canceled, allowed=false, full burst afterwards); live in-flight cancel through the holding proxy (asserts Canceled, allowed=false, returns within half the hold) and logs whether the token was charged. Do NOT use a time-based busy Lua loop: Redis 7 freezes TIME inside scripts and the loop never ends.

## Tests
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik14173_MemoryCanceledContext and TestTraefik14173_RedisCanceledInFlight (uses the holding TCP proxy countingProxy/forwardHeld) -> tokenbucket/traefik_issue14173_test.go and tokenbucket/traefik_issue14173_e2e_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).
Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

## Problem

Traefik responde 500 cuando el cliente se va porque su `select` no distingue cancelación. En esta librería el 500 no se da: `Allow` ya devuelve `context.Canceled` con `Allowed` falso. Falta documentar (GoDoc) que un `Canceled`/`DeadlineExceeded` de Redis no prueba que el script no haya corrido, y faltar los tests de este issue (pre-cancelado en Memory; cancelación en vuelo en Redis vivo).

## Current (code)

- `tokenbucket/memory.go:65-70` — GoDoc de `Memory.Allow` no dice que un ctx ya terminado devuelve su error y no cobra. El cuerpo hace `ctx.Err()` antes del mutex y del consume; si hay error, `Status{}` y ese error.
- `tokenbucket/redis.go:52-70` — GoDoc de `Redis.Allow` no dice que `Canceled`/`DeadlineExceeded` no prueba que el script no haya corrido. El cuerpo hace `ctx.Err()` y, si el ctx sigue vivo, llama `Eval`; el error de `Eval` se propaga con `Status{}`.
- `tokenbucket/status.go:11-12` — la librería no escribe HTTP ni duerme. El mapeo 499 / no esperar si `wait==0` no está aquí: `not found` en este paquete.
- `tokenbucket/status.go:13-15` — `Status.Allowed` es el `allowed` del diseño; un error no nulo deja el `Status` a cero (`Allowed` falso).
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — escenario «Error returns the zero status»: contexto cancelado → `Status` cero. No documenta el cobro Redis en vuelo.
- `knowledge/devdocs/std_go_tokenbucket.md:28-47` — el snippet trata cualquier error de `Allow` igual; no cita #14173 ni el cobro en vuelo.
- `tokenbucket/status_test.go:164-172` — `TestStatus_ErrorReturnsTheZeroStatus` afirma `context.Canceled` y `Status` cero en Memory y Redis. No afirma el burst completo después ni la cancelación en vuelo.
- `tokenbucket/limiter_e2e_test.go:73-87` — `runForEachLiveEngine` y `TestLive_Limiter` (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`); skip con `-short` o sin env.
- `tokenbucket/limiter_e2e_test.go:209-227` — `waitLiveClient`.
- `simpleredis/commands_eval.go:28-35` — `Eval` manda `EVALSHA` vía `exec(ctx, …)`.
- `simpleredis/commands_exec.go:21-32` — `exec` propaga `ctx.Err()`; el comentario admite que EVAL puede aplicar dos veces si se pierde la respuesta.
- `tokenbucket/traefik_issue14173_test.go` — not found
- `tokenbucket/traefik_issue14173_e2e_test.go` — not found
- `countingProxy` / `forwardHeld` / `liveKey` / `countAdmits` en este árbol — not found (el repro de referencia vive fuera, en el worktree de análisis, con la firma antigua de `Allow`).

## Desired

- Sin cambio de comportamiento.
- GoDoc en `Redis.Allow`: un retorno `context.Canceled` / `DeadlineExceeded` no prueba que el script no haya corrido; el token puede haberse cobrado.
- GoDoc en `Memory.Allow`: un ctx ya terminado devuelve su error y no cobra.
- Mencionar que el mapeo HTTP (499 si `Canceled`, no esperar si `wait==0`) es del middleware, citando traefik/traefik#14173.
- `tokenbucket/traefik_issue14173_test.go`: `TestTraefik14173_MemoryCanceledContext` afirma `Canceled`, `allowed=false`, burst completo después.
- `tokenbucket/traefik_issue14173_e2e_test.go`: `TestTraefik14173_RedisCanceledInFlight` con proxy TCP que retiene escrituras; afirma `Canceled`, `allowed=false`, retorno en menos de la mitad del hold; registra si se cobró el token. Misma puerta que `TestLive_Limiter`. No usar un bucle Lua ocupado con `TIME`.
- Helpers propios con sufijo `14173` en esos ficheros. No helpers compartidos. No copiar tests de otros issues.
- La tarjeta de entrega y el cuerpo del PR nombran los issues #14173 y #13879 y los PRs #13880 y #14236.

## Affected

- `tokenbucket/memory.go` (GoDoc de `Allow`)
- `tokenbucket/redis.go` (GoDoc de `Allow`)
- `tokenbucket/traefik_issue14173_test.go` (nuevo)
- `tokenbucket/traefik_issue14173_e2e_test.go` (nuevo)

## Out of scope

- Cambiar el comportamiento de `Allow` (devolver otro error, reembolsar el token cobrado en vuelo, no enviar EVAL si el ctx se cancela).
- Escribir 499 u otro HTTP desde la librería.
- Reescribir el `select` / timer del middleware Traefik.
- El resto de issues upstream (HMGET, TTL, ámbito, cliente compartido, etc.).
- Helpers compartidos entre issues, o copiar tests de otros issues.
- Un bucle Lua ocupado basado en `TIME` para bloquear el servidor.
- `simpleredis` salvo usarlo como está; no cambiar `Eval`/`exec`.

## Unknowns

- Si Dragonfly 1.40.2 sigue sin ejecutar el EVAL retenido en este árbol. Eso se midió el 2026-11-22 en el worktree de análisis; aquí no se ha vuelto a ejecutar.
- Si el spec vivo necesita un delta solo por GoDoc. El diseño no pide cambio de comportamiento; propose transcribe, no rediseña.
- Cómo adaptar al `Status` actual los asserts del repro de referencia (`allowed, wait, err := limiter.Allow`). Dest ya devuelve `(Status, error)`.

## Tensions

- El repro de referencia (`traefik_issues_test.go` / `traefik_issues_e2e_test.go` en el worktree de análisis) llama `Allow` con la firma `(bool, time.Duration, error)`. Dest (`tokenbucket/memory.go`, `tokenbucket/redis.go`) devuelve `(Status, error)` tras el PR #106 de este repo. Los tests nuevos deben usar `status.Allowed`, no reintroducir la firma vieja.
- El 500 de Traefik no se reproduce en la librería (tabla B1 / tests de referencia pasan). El ticket es TEST+DOC; el único efecto lateral (cobro Redis en vuelo) se documenta, no se arregla.
- `TestStatus_ErrorReturnsTheZeroStatus` ya cubre ctx pre-cancelado y `Status` cero, pero no el burst completo ni la cancelación en vuelo. El ticket pide tests propios de este issue, no tautologías; ese test existente no sustituye a `TestTraefik14173_*`.
- `knowledge/devdocs/std_go_tokenbucket.md` no cita el cobro en vuelo. El ticket pide GoDoc en `Allow`, no un cambio del paquete de uso; el impacto en devdocs lo mide otra fase.
