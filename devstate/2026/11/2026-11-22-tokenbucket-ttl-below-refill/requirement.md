# Requirement
IssueKey: 2026-11-22-tokenbucket-ttl-below-refill

# Requirement (FIX (reproduced in Memory, Redis and Dragonfly))

Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13957

Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

Root cause: an idle bucket that expires (Memory delete / Redis EXPIRE) is recreated full. When ttl < refill time, a client that pauses longer than ttl gets the missing tokens for free (30 req/min, burst 30, Traefik's ttl=3s: 30 admitted after a 5 s pause instead of 2).

Binding design (tech-lead decision, do not redesign):

- NewMemory and NewRedis raise the effective TTL to a floor: refillFloor = ceil(burst/rate + maxDelay.Seconds()) whole seconds. effectiveTTL = max(callerTTL, refillFloor). Never lower the caller's TTL. Rationale: TTL only exists to reclaim idle state; at or above the floor an expired bucket is already full, so recreating it full grants nothing. Raising never changes admissions, only how long idle keys live. Rejecting short TTLs was considered and rejected because it breaks callers that pass Traefik's own formula.
- validateClock keeps its current contract (ttl >= 1s, whole seconds) on the caller's value. The floor is applied after validation and stored in clockConfig.ttl, so Memory expiry and Redis EXPIRE (ttlSeconds) keep using the same value (BUGS.md bug 4 invariant).
- Overflow: burst/rate can be enormous (tiny rate). Cap the floor so the result never overflows time.Duration (clamp to the largest whole-second time.Duration) and test that case.
- GoDoc on NewMemory/NewRedis: ttl is a minimum; the library raises it to the refill time and why (cite traefik/traefik#13957).
- Tests: the two repro tests must pass after the fix (traefik_ttl case now admits 2 in Memory and <=3 live); keep the control case. Add unit tests for the floor: short ttl raised, long ttl kept, maxDelay included, overflow clamp.

## Tests

Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13957_MemoryIdleBucketRecreatedFull (traefik_issues_test.go) and TestTraefik13957_RedisIdleBucketRecreatedFull (traefik_issues_e2e_test.go) -> tokenbucket/traefik_issue13957_test.go and tokenbucket/traefik_issue13957_e2e_test.go.

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).

Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

## Upstream

La existencia del defecto se toma de los informes upstream, no de un hallazgo local inventado.

- Issue: https://github.com/traefik/traefik/issues/13957
- Pull request upstream: ninguno. La sección A2 de `TRAEFIK-UPSTREAM-ISSUES.md` no cita un PR; el issue sigue abierto.

## Current (code)

- `tokenbucket/memory.go` `NewMemory` guarda el `ttl` del llamador en `clockConfig.ttl` tras `validateClock`. No sube ningún suelo.
- `tokenbucket/redis.go` `NewRedis` hace lo mismo: `clockConfig.ttl` es el `ttl` recibido.
- `tokenbucket/clock.go` `validateClock` rechaza `ttl < 1s` o con fracción de segundo (`errTTL`). No compara el `ttl` con `burst/rate`.
- `tokenbucket/clock.go` `ttlSeconds` es `int64(ttl / time.Second)`. Con segundos enteros, Memory y Redis usan la misma vida.
- `tokenbucket/memory.go` `Allow`: si `expireAt` ya pasó, borra la entrada y crea `memEntry` con `tokens = burst`.
- `tokenbucket/lua.go` `allowScript`: si `HGETALL` no devuelve 4 campos, siembra `tokens = burst` y `last = t`. `EXPIRE` usa el ARGV de segundos.
- `tokenbucket/redis.go` `Allow` pasa `ttlSeconds()` como ARGV del script.
- `openspec/specs/std_go_tokenbucket_allow/spec.md`: una clave ausente o con `ttl` cumplido empieza llena, y las entradas caducan tras el `ttl` del llamador.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`: el mismo contrato de construcción y de segundos enteros para Redis.
- `tokenbucket/limiter_e2e_test.go` `runForEachLiveEngine`, `waitLiveClient` y `TestLive_Limiter` existen y son la compuerta de los tests en vivo.
- `tokenbucket/traefik_issue13957_test.go` — not found
- `tokenbucket/traefik_issue13957_e2e_test.go` — not found
- `tokenbucket/BUGS.md` — not found en `origin/master` (el invariante del bug 4 ya está en `tokenbucket/clock.go`).

## Out of scope

- Rechazar un `ttl` corto en `validateClock` (el diseño lo descarta).
- Cambiar la fórmula de admisión (`consumeOne`, el script Lua) o el relleno desde epoch.
- El resto de issues upstream (cap de fuentes, 499, HMGET, scope, cliente compartido).
- Cambiar `simpleredis` o el cálculo de `ttl` dentro de Traefik.
- Bajar el `ttl` que pase el llamador.

## Unknowns

- Qué tests de dest afirman que el `ttl` guardado es exactamente el del llamador. El radio de esos asserts no está medido aquí.
- El valor numérico del tope de `time.Duration` en segundos enteros en este toolchain. El diseño pide el mayor segundo entero que quepa; no fija el literal.
- Los números del repro en este worktree. Los tests de referencia viven en otro checkout y aún no están aquí.

## Tensions

- El spec vivo (`openspec/specs/std_go_tokenbucket_allow/spec.md`) dice que las entradas caducan tras el `ttl` del llamador. El diseño guarda `max(callerTTL, refillFloor)` en `clockConfig.ttl`. La caducidad efectiva puede ser mayor que el argumento. El cambio de spec tiene que decirlo; el código de dest todavía no lo hace.
- El archivo de diseño archivado `openspec/changes/archive/2026-09-13-tokenbucket-ttl-whole-seconds/design.md` rechazó un suelo silencioso de un `ttl` fraccionario mientras `New` tenía éxito. Ese suelo era truncar segundos. Este ticket sube un `ttl` ya válido (segundos enteros) hasta el tiempo de rellenado, después de `validateClock`. No es el mismo mecanismo; el contrato de `validateClock` se mantiene.
