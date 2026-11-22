# Requirement

IssueKey: 2026-11-22-tokenbucket-hgetall-field-order

## Upstream

La existencia del defecto se toma de los informes upstream, no de un hallazgo local independiente:

- Issue: https://github.com/traefik/traefik/issues/14297
- Fix upstream: https://github.com/traefik/traefik/pull/14300

Análisis de ese issue: sección A1 de `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md`.

## Spec

# Requirement (FIX (reproduced on live Redis and Dragonfly))

Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/14297 ; upstream fix PR https://github.com/traefik/traefik/pull/14300
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

Root cause: tokenbucket/lua.go reads HGETALL by position (rl_source[2]=last, rl_source[4]=tokens). Redis/Valkey do not guarantee hash field order (hashtable encoding order depends on the per-process hash seed). When tokens comes first the bucket refills on every call and the limiter never limits (reproduced: 10 of 10 admitted with burst 5).

Binding design:

- In allowScript replace HGETALL + `#rl_source == 4` with `redis.call('hmget', key, 'last', 'tokens')`; treat the bucket as existing only when both values are truthy (HMGET returns false for a missing field on Redis; verify Dragonfly Lua 5.4 behaves the same in the live test); otherwise keep the current seed branch (tokens = burst, last = t).
- Same approach as upstream PR #14300. Update the header comment of lua.go: the script is copied from Traefik @903e8a9 and now deviates in this read, citing traefik/traefik#14297 and PR #14300. Keep the rest of the Traefik script unchanged.
- allowScriptDigest is derived from allowScript; no manual change.
- Memory store is unaffected; do not touch it.
- The repro test must pass after the fix on both engines: control_last_first, tokens_first_listpack and tokens_first_hashtable (hashtable may SKIP when the server's hash seed keeps last first; that is expected and documented in the test).
- Existing TestLive_Limiter and Yaegi live tests must keep passing (the script runs under Yaegi too).

## Tests

Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik14297_HGetAllFieldOrder (tokenbucket/traefik_issues_e2e_test.go) -> new file tokenbucket/traefik_issue14297_e2e_test.go.

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).

Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

## Problem

`tokenbucket/lua.go` lee el hash del bucket con `HGETALL` y asigna `rl_source[2]` a `last` y `rl_source[4]` a `tokens`. Redis y Valkey no garantizan el orden de los campos. Si `tokens` llega primero, el bucket se rellena en cada llamada y el limitador no limita (reproducido: 10 de 10 admitidas con burst 5).

## Current (code)

- `tokenbucket/lua.go`: `allowScript` hace `redis.call('hgetall', key)` y, si `#rl_source == 4`, toma `last` de `rl_source[2]` y `tokens` de `rl_source[4]`. Si no, siembra `tokens = burst` y `last = t`. El comentario de cabecera dice que el script está copiado de Traefik @903e8a9 con `table.maxn` sustituido por `#rl_source == 4`.
- `tokenbucket/redis.go`: `allowScriptDigest` es `simpleredis.ScriptSHA1Hex(allowScript)` en init; `Redis.Allow` evalúa ese script. No hay digest escrito a mano.
- `tokenbucket/memory.go`: `Memory.Allow` no usa Redis ni el script.
- `tokenbucket/limiter_e2e_test.go`: `TestLive_Limiter` entra por `runForEachLiveEngine` (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`) y `liveEngineAddrs` hace skip con `-short` o sin esas variables. `waitLiveClient` está en el mismo archivo.
- `tokenbucket/limiter_yaegi_e2e_test.go`: `TestYaegiLive_RedisAndDragonfly` ejecuta el limitador bajo Yaegi contra los motores vivos.
- `tokenbucket/traefik_issue14297_e2e_test.go`: not found.
- `tokenbucket/traefik_issues_e2e_test.go`: not found en este árbol (el repro de referencia vive fuera del repo, en el worktree de análisis).

## Desired

- Sustituir en `allowScript` el `HGETALL` y el `#rl_source == 4` por `redis.call('hmget', key, 'last', 'tokens')`. El bucket existe solo si ambos valores son truthy; si no, conservar la siembra actual (`tokens = burst`, `last = t`).
- Mismo enfoque que el PR upstream #14300. Actualizar el comentario de cabecera de `tokenbucket/lua.go`: copiado de Traefik @903e8a9 y desviado en esta lectura, citando traefik/traefik#14297 y el PR #14300. El resto del script de Traefik no cambia.
- No tocar `allowScriptDigest` a mano ni `tokenbucket/memory.go`.
- Añadir `tokenbucket/traefik_issue14297_e2e_test.go`, autónomo, con `TestTraefik14297_HGetAllFieldOrder` y helpers propios con sufijo `14297`. Debe cubrir `control_last_first`, `tokens_first_listpack` y `tokens_first_hashtable` (este último puede hacer SKIP si la semilla deja `last` primero). Afirmar el comportamiento de un limitador correcto. Misma puerta que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient`.
- `TestLive_Limiter` y los tests Yaegi vivos del paquete siguen pasando.
- La tarjeta de entrega y el cuerpo del PR nombran el issue #14297 y el PR #14300.

## Affected

- `tokenbucket/lua.go`
- `tokenbucket/traefik_issue14297_e2e_test.go` (nuevo)
- `tokenbucket/redis.go` solo en cuanto el digest se deriva del script; sin edición manual

## Out of scope

- Cambiar el almacén en memoria (`tokenbucket/memory.go`).
- Reescribir el resto del script (refill, max_delay, HSET, EXPIRE).
- Helpers compartidos entre issues, o copiar tests de otros issues.
- `simpleredis` y sus motores de auth (`SIMPLEREDIS_LIVE_*`), salvo que un test de este issue los necesitara; no los necesita.
- Fusionar o modificar el PR upstream.

## Unknowns

- Si Dragonfly (Lua 5.4) devuelve `false` en `HMGET` para un campo ausente, igual que Redis. Hay que comprobarlo en el test vivo; si devolviera `nil`, la condición truthy de ambos valores seguiría siendo válida, pero no está medido en este árbol.
- Si la semilla de hash del Redis local deja `last` primero, el subtest hashtable hará SKIP. Eso es el comportamiento esperado del test, no un fallo, y no está fijado hasta ejecutarlo.

## Tensions

Ninguna entre el ticket y el código. El diseño vinculante manda desviarse de la copia literal de Traefik en esta lectura y documentarlo en el comentario de cabecera.
