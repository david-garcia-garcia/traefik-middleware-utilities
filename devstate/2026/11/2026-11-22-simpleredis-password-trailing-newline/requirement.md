# Requirement
IssueKey: 2026-11-22-simpleredis-password-trailing-newline

El texto del solicitante es un diseño de producto completo (`ticket/source.md`, spec del llamante con `issueHost: local`). Se copia íntegro a continuación; las secciones posteriores (`Current (code)`, `Out of scope`, `Unknowns`, `Tensions`) son el análisis de prepare y no sustituyen al spec. El binding de diseño lo decide el tech lead; propose lo transcribe, no lo rediseña.

---

# simpleredis: AUTH password trailing newline (traefik/traefik#12078)

issueHost: local
issueRef: none
Requirement type: TEST + DOC (cause confirmed in this client).

## Upstream

The existence of the defect was taken from these upstream reports:

- Issue https://github.com/traefik/traefik/issues/12078 (cause identified in its comments: Kubernetes Secret values ending in a newline)

Full analysis of all upstream issues: `tokenbucket/TRAEFIK-UPSTREAM-ISSUES.md` in the read-only worktree `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros` (section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

No behavior change (trimming a password would be a silent security-relevant transform). Test on the requirepass engines (`SIMPLEREDIS_LIVE_REDIS_AUTH` / `_DRAGONFLY_AUTH`, password `"secret"`): the exact password authenticates, `"secret"` plus a trailing newline fails with `redis:noauth`. GoDoc on `Config.Pass`: sent to AUTH byte for byte, no trimming; a trailing newline from a Secret or file makes AUTH fail (cite traefik/traefik#12078).

## Tests

Reference repro tests already written and run against live engines in `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros`. Use: `TestTraefik12078_PasswordWithTrailingNewline` (`simpleredis/traefik_issue12078_e2e_test.go`, copy as is).

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. `countAdmits14297`, `liveKey14297`) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as `TestLive_Limiter` (skip under `-short` or without the env vars) via the existing `runForEachLiveEngine`/`waitLiveClient` helpers in `tokenbucket/limiter_e2e_test.go` (simpleredis has its own `runForEachLiveEngine`/`newTestRedis`).

Local engines already running in Docker: `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` `SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381` `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382` (password `secret`). Local gate before pushing: `go vet ./...` ; `go test -short -count=1 ./...` ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.

---

## Current (code)

Base: `origin/master` @ `7a435ac246876370b0203890e57534a686592cb6`.

- `Config.Pass` GoDoc dice «AUTH on each new dial when non-empty»; no dice que se envíe byte a byte ni que no se recorte: `simpleredis/config.go` (`type Config`, campo `Pass`).
- `New` copia `cfg.Pass` a `sr.pass` sin `Trim` ni otra transformación: `simpleredis/simpleredis.go` (`func New`).
- `authArgs` construye `[]byte(sr.pass)` y lo manda en AUTH; no hay recorte: `simpleredis/pool.go` (`func (sr *SimpleRedis) authArgs`).
- `applyDefaults` no toca `Pass`: `simpleredis/config.go` (`func (cfg Config) applyDefaults`).
- `TestTraefik12078_PasswordWithTrailingNewline` — `not found`.
- `simpleredis/traefik_issue12078_e2e_test.go` — `not found`.
- e2e AUTH live cubre WRONGPASS con `"wrong-password"`, no el caso `"secret\n"`: `simpleredis/pool_e2e_test.go` (`TestLive_WrongPassword`).
- Fake AUTH cubre forma de argumentos con `Pass` `"secret"` exacto; no hay fila con salto de línea: `simpleredis/pool_test.go` (`TestAuthArgumentForms`).
- Devdoc de uso no documenta recorte ni newline en `Pass`: `knowledge/devdocs/std_go_simpleredis.md`.
- Contrato live e2e exige WRONGPASS → `redis:noauth` en motores AUTH; no nombra trailing newline: `openspec/specs/std_go_simpleredis_live-e2e/spec.md` (Requirement: Live Redis and Dragonfly prove engine-success verbs, Scenario: Live WRONGPASS).
- Contrato de sesión exige `AUTH <pass>` cuando `User` está vacío; el escenario usa `Pass` `secret` y no promete bytes literales ni newline: `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (Requirement: AUTH and SELECT run once per dial).
- Gate live del paquete (skip bajo `-short` o sin env): `simpleredis/simpleredis_e2e_test.go` (`runForEachLiveEngine`, `liveEngineAddrs`).
- Constructor de cliente en tests: `simpleredis/newtest_test.go` (`newTestRedis`).

## Out of scope

Pedido o decidido por el binding (se listan, no se toman):

- Recortar `Config.Pass` (o `User`) antes de AUTH: el binding lo prohíbe; sería un transform silencioso con relevancia de seguridad.
- Cambiar el comportamiento de AUTH en Redis/Dragonfly o en Traefik upstream.
- Copiar tests de otros issues o crear ficheros de helpers compartidos nuevos.

Inferido por prepare, no pedido:

- Cambiar `tokenbucket/` o `windowcounter/`.
- Recortar `\r\n` u otros whitespace distintos del `\n` del test de referencia.
- Añadir un escenario OpenSpec si el ticket se cubre solo con test + GoDoc (propose transcribe; no se inventa un contrato extra aquí).

## Unknowns

- Si propose debe añadir un escenario al catálogo `openspec/specs/` además del GoDoc: el ticket es TEST + DOC y no nombra un spec id.
- Si un Secret o fichero con `\r\n` (Windows) debe cubrirse: el repro de referencia solo usa `"secret\n"`.
- Qué hace Traefik / go-redis con el mismo Secret: fuera de este cliente; la causa en simpleredis está confirmada en el repro live (`TestTraefik12078_PasswordWithTrailingNewline` en el worktree de referencia).

## Tensions

- Binding «no behavior change» frente al issue upstream, donde el síntoma en Kubernetes se arreglaría recortando el newline del Secret. Aquí se documenta y se prueba el envío byte a byte; no se recorta.
- «Helpers con sufijo del issue» frente a «copy as is»: el test de referencia ya usa `runForEachLiveEngine` / `newTestRedis` del paquete (`simpleredis/simpleredis_e2e_test.go`, `simpleredis/newtest_test.go`). El spec resuelve: copiar el test tal cual y reutilizar esos helpers existentes; no duplicarlos ni crear helpers compartidos nuevos.
- TEST + DOC frente al contrato vivo, que no promete bytes literales ni cita traefik/traefik#12078. Propose transcribe el binding; no rediseña. Un escenario de catálogo extra no está pedido.
