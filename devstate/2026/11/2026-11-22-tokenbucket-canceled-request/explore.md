# Explore

## Concepts

**Unidades**

| Unidad | Ruta | Trabajo |
|--------|------|---------|
| `Memory.Allow` | `tokenbucket/memory.go` | Consume in-process. `ctx.Err()` antes del mutex; si hay error, `Status{}` y ese error, sin cobro. |
| `Redis.Allow` | `tokenbucket/redis.go` | Consume vía `Eval`. `ctx.Err()` y, si el ctx sigue vivo, `EVALSHA`; el error de `Eval` sale con `Status{}`. |
| `Status` | `tokenbucket/status.go` | Resultado de un Allow. La librería no escribe HTTP ni duerme. Error no nulo → Status cero (`Allowed` falso). |
| `SimpleRedis.Eval` / `exec` / `do` | `simpleredis/commands_eval.go`, `commands_exec.go`, `resp.go` | Manda EVALSHA. `watchConnClose` cierra el socket al cancelar el ctx. Fuera de alcance cambiarlo. |
| Tests de este issue | `tokenbucket/traefik_issue14173_test.go`, `traefik_issue14173_e2e_test.go` | Aún no existen en este árbol. |
| Contrato vivo | `openspec/specs/std_go_tokenbucket_allow/spec.md` | Escenario «Error returns the zero status»: ctx cancelado → Status cero. No documenta el cobro Redis en vuelo. |
| Uso | `knowledge/devdocs/std_go_tokenbucket.md` | Snippet: cualquier error de `Allow` igual. No cita #14173 ni el cobro en vuelo. |
| Control existente | `tokenbucket/status_test.go` `TestStatus_ErrorReturnsTheZeroStatus` | Afirma `context.Canceled` y Status cero en Memory y Redis fake. No afirma burst completo ni cancelación en vuelo. |
| Puerta live | `tokenbucket/limiter_e2e_test.go` `runForEachLiveEngine`, `waitLiveClient` | Misma puerta que `TestLive_Limiter` (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`; skip con `-short` o sin env). |

**Conexión**

```text
Caller ctx ──► Memory.Allow ──► ctx.Err()? ──yes──► Status{} + err (no cobra)
                    │ no
                    ▼ mutex + consumeOne

Caller ctx ──► Redis.Allow ──► ctx.Err()? ──yes──► Status{} + err (no cobra)
                    │ no
                    ▼ Eval / EVALSHA (simpleredis.do)
                         │
              ctx cancel mid-I/O ──► watchConnClose ──► Canceled pronto
                         │
                         ▼ bytes ya en el cable / proxy que retiene
                   el servidor puede ejecutar el script igual
                   (2026-11-22: Redis cobró; Dragonfly no)

Middleware (fuera de este paquete):
  errors.Is(err, context.Canceled) ──► 499, no 500
  wait == 0 ──► no hacer select
```

La clave opaca y el mapeo HTTP siguen en el caller. Este cambio no reconstruye dirección, usuario, tenant ni Host.

**Call sites de `Memory.Allow` / `Redis.Allow` (enumerados)**

- Raíces: `tokenbucket/**/*.go` patrón `.Allow(`; `README.md`; `knowledge/devdocs/std_go_tokenbucket.md`; `**/*.go` para otros `Allow` (descartados: `windowcounter.Limiter.Allow`, `backendbackoff.Gate.Allow`).
- Definiciones: 2 — `tokenbucket/memory.go:67`, `tokenbucket/redis.go:54`. Firma Dest: `(Status, error)`.
- Invocaciones `.Allow(` en tests/bench de `tokenbucket/`: **82** (ninguna en `limiter_yaegi_e2e_test.go`).
- Fuera del paquete en este árbol: `README.md` (1 ejemplo), `knowledge/devdocs/std_go_tokenbucket.md` (1 snippet).
- Ningún importador de producción de `tokenbucket.Allow` fuera de tests/docs en el worktree (es librería).
- Este cambio no migra esas 82 llamadas: GoDoc + tests nuevos. `traefik_issue14173_*.go` — not found aquí.

**Reproducir**

El 500 de Traefik **not reproduced** en la librería.

- Código Dest: `Memory.Allow` (`memory.go:68-70`) y `Redis.Allow` (`redis.go:55-57`) devuelven `ctx.Err()` con `Status{}` antes de consumir. `status.go:11-12`: no HTTP. `simpleredis/resp.go:38-39,121-138`: cancel en vuelo cierra el conn (`watchConnClose`); `writeCommand` puede haber salido ya.
- Camino ejecutado en este worktree:

```text
go test -short -count=1 -v -run '^TestStatus_ErrorReturnsTheZeroStatus$' ./tokenbucket/
```

  **PASS** (0.02s). Memory y Redis fake: `errors.Is(err, context.Canceled)` y Status cero.
- Cancelación en vuelo / cobro Redis: **not reproduced** en este árbol en esta fase (no hay `TestTraefik14173_*`). Evidencia 2026-11-22 en el worktree de análisis: `TestTraefik14173_MemoryCanceledContext` (`tokenbucket/traefik_issues_test.go`) y `TestTraefik14173_RedisCanceledInFlight` (`tokenbucket/traefik_issues_e2e_test.go`; helpers `countingProxy` / `forwardHeld`, no ficheros `traefik_issue14173_*.go`). Redis: `tokens = 2` de burst 3 (cobró). Dragonfly: hash vacío (no ejecutó). Allow volvió en ~200 ms con `Canceled`. No se usó bucle Lua con `TIME` (Redis 7 lo congela).

**Hechos externos**

- In-tree: `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (error → 500; `time.After(*delay)`); `knowledge/research/ext_redis_eval/`; spec y usage citados arriba.
- Análisis (lectura): `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md` sección B1 y hallazgo 3.
- Upstream (existencia del defecto): https://github.com/traefik/traefik/issues/14173 https://github.com/traefik/traefik/issues/13879 https://github.com/traefik/traefik/pull/13880 https://github.com/traefik/traefik/pull/14236

## Decisions

- **Enfoque:** Sin cambio de comportamiento. GoDoc en `Redis.Allow`: `Canceled` / `DeadlineExceeded` no prueba que el script no corriera; el token puede haberse cobrado. GoDoc en `Memory.Allow`: ctx ya terminado → su error y no cobra. Mencionar que el mapeo HTTP (499 si `Canceled`, no esperar si `wait==0`) es del middleware, citando traefik/traefik#14173. Tests propios: Memory pre-cancelado (`Canceled`, `Allowed` falso, burst completo después); live in-flight con proxy TCP que retiene escrituras (`Canceled`, `Allowed` falso, retorno en menos de la mitad del hold) y log del cobro. Helpers con sufijo `14173` en esos ficheros. Reusar `runForEachLiveEngine` / `waitLiveClient`.
- **Rechazado:** Cambiar `Allow` (otro error, reembolsar, no enviar EVAL) — Out of scope. Escribir 499 desde la librería — Out of scope. Reescribir el `select` de Traefik — Out of scope. Bucle Lua ocupado con `TIME` — Out of scope. Helpers compartidos o copiar tests de otros issues — Out of scope. Tocar `simpleredis` `Eval`/`exec` — Out of scope. Rediseñar en propose — el diseño es binding.
- **Contrato vivo:** `openspec/specs/std_go_tokenbucket_allow/spec.md` (familia `std` / `go` / `tokenbucket` en `openspec/specs/map.md`). También `std_go_tokenbucket_status` y `std_go_tokenbucket_lua-eval`; este ticket no los reescribe.

## Open questions

- Q: ¿Dragonfly 1.40.2 sigue sin ejecutar el EVAL retenido en este árbol?
  Rank: additive asked — no reshape de callers; Unknowns nombra la medición; Desired pide registrar si se cobró, no un assert por motor
  Decision: assumed — el test live registra el bucket tras el hold y no falla por cobro vs vacío. Medición 2026-11-22 (análisis): Redis cobró (`tokens=2` de 3); Dragonfly vacío.
  By: explore

- Q: ¿El spec vivo `std_go_tokenbucket_allow` necesita un delta solo por GoDoc?
  Rank: additive incidental — un escenario nuevo documentaría el contrato existente; Desired nombra GoDoc y tests, no un leaf
  Decision: assumed — propose transcribe. Si hay delta, documenta que `Canceled`/`DeadlineExceeded` de `Redis.Allow` no prueba que el script no corrió; no añade SHALL de reembolso ni de retener EVAL. «Error returns the zero status» ya cubre Status cero.
  By: explore

- Q: ¿Cómo adaptar los asserts del repro de referencia a `(Status, error)`?
  Rank: additive asked — Dest es `(Status, error)` en `memory.go:67` y `redis.go:54`; Desired afirma `allowed=false`; Tensions: no reintroducir la firma vieja
  Decision: resolved — `!status.Allowed` y Status cero si `err != nil`; burst completo con Allows posteriores y `status.Allowed`. No `(bool, time.Duration, error)`.
  By: explore

- Q: ¿Quién mapea HTTP 499 y omite la espera si `wait==0`?
  Rank: additive asked — Desired: el mapeo HTTP es del middleware, citando #14173; Out of scope: escribir 499 desde la librería
  Decision: resolved — el middleware. GoDoc de Allow lo menciona y cita traefik/traefik#14173. `tokenbucket` no escribe HTTP (`status.go:11-12`).
  By: explore

- Q: ¿Dónde viven `countingProxy`, `liveKey` y `countAdmits`?
  Rank: additive asked — Tests: helpers propios con sufijo 14173; Out of scope: helpers compartidos y copiar tests de otros issues
  Decision: resolved — `countingProxy14173`, `forwardHeld14173`, `startCountingProxy14173`, `liveKey14173`, `evalKey14173`, `countAdmits14173` en `tokenbucket/traefik_issue14173_test.go` y `tokenbucket/traefik_issue14173_e2e_test.go`. Reusar `runForEachLiveEngine` y `waitLiveClient` de `limiter_e2e_test.go`. No copiar `traefik_issues_test.go`.
  By: explore

- Q: ¿El test live debe afirmar cobro en Redis y no cobro en Dragonfly?
  Rank: additive asked — Desired: registra si se cobró; hallazgo 3 del análisis lo trata como log
  Decision: resolved — loguear el estado del bucket; no `Fatal` por tokens cobrados vs hash vacío.
  By: explore

- Q: ¿`TestStatus_ErrorReturnsTheZeroStatus` sustituye a `TestTraefik14173_*`?
  Rank: additive asked — Tensions: no afirma burst completo ni cancelación en vuelo; Desired nombra tests propios
  Decision: resolved — no. Tests nuevos autocontenidos. El existente se queda como control de Status cero.
  By: explore

- Q: ¿Cambiar `Allow` / `Eval` / `exec` para no enviar o reembolsar si el ctx se cancela en vuelo?
  Rank: additive asked — Desired «Sin cambio de comportamiento»; Out of scope nombra reembolsar y no enviar EVAL
  Decision: resolved — no. GoDoc del efecto lateral. `simpleredis` intacto.
  By: explore

- Q: ¿Actualizar `knowledge/devdocs/std_go_tokenbucket.md` en esta entrega?
  Rank: additive incidental — el packet no cita el cobro en vuelo; Desired nombra GoDoc en Allow, no el packet; Tensions: el impacto lo mide otra fase
  Decision: assumed — no escribir el packet en explore ni en el alcance mínimo de apply; `opd-devdocsimpact` lo mide después.
  By: explore
