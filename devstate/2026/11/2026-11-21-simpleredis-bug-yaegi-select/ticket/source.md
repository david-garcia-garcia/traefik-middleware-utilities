# simpleredis: quitar los `select` que yaegi v0.16.1 cruza entre goroutines

Origen: conversación con el desarrollador (2026-11-21). issueHost local. Pide implementación + PR, y sobre todo cobertura de test que pruebe la versión **interpretada** (yaegi).

## Problema (comprobado)

Yaegi v0.16.1 (`go.mod`), `interp/run.go` `func _select`: reserva **un único** `cases []reflect.SelectCase` por sentencia `select` y cada ejecución lo sobrescribe sin lock antes de `reflect.Select(cases)`. Dos goroutines que ejecutan la misma sentencia con canales distintos pueden esperar en, o recibir de, los canales de la otra. Mismo mecanismo que crowdsec-bouncer-traefik-plugin PR #208 (https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/pull/208), que lo resolvió en `pkg/lapi` sustituyendo `select` por `time.AfterFunc`.

Sentencias `select` de producción en este repo:

- `simpleredis/pool.go` `freeInUseTurn` — envío no bloqueante a `sr.inUseTurns`.
- `simpleredis/pool.go` `borrowSocket` — toma no bloqueante de `sr.inUseTurns`, y espera bloqueante sobre `sr.inUseTurns`, `ctx.Done()` y `timer.C` (PoolTimeout).
- `simpleredis/commands_exec.go` `waitUntil` — backoff entre reintentos sobre `ctx.Done()` y `timer.C`.
- `reclaim/table.go` `waitCtx` — `finished` y `tick.C`. Solo con contextos cuyo `Done()` es nil (Background, que nunca se cancela): impacto bajo.
- `windowcounter` ya usa `time.AfterFunc` (comentario en `limiter.go` `startFlushLocked`); no afectado.

## Reproducción (ya hecha, archivos sin commit en el checkout principal)

`D:\repositories\traefik-middleware-utilities\simpleredis\`:

- `repro_yaegi_select_probe_test.go` — sondas sin import de `testing` que llaman al código interno real (`waitUntil`, `borrowSocket`, semáforo). Se ejecutan compiladas y, copiadas al paquete interpretado como `zz_selectprobe.go`, bajo yaegi.
- `repro_yaegi_select_crosstalk_test.go` — pares `TestReproSelect_*_Compiled` / `_Yaegi`.

Resultados (compilado siempre limpio; yaegi falla 4 de 4 / 3 de 3):

| Efecto | yaegi |
|---|---|
| `borrowSocket` devuelve `(nil, nil)`; vía `Get` → `runOnConn` con `conn == nil` → panic en `resp.go:19` + `release`→`close`; tumbó el proceso de test | 2–13 por ejecución |
| Fuga permanente de turnos entre dos clientes + `simpleredis_over_free` falsos | 1–5 de 32 turnos perdidos en 0,4 s |
| `waitUntil` de 1 ms acaba en `DeadlineExceeded` (tick robado) | 14–27 por ejecución |
| Espera de pool que despierta con el canal de otra llamada | 0–2 por ejecución |
| Cuelgue en `<-timer.C` tras `Stop()` con tick robado | no reproducido (posible por código) |

Inferencia no reproducida: `windowcounter` `flushAfterTick` llama a Redis desde una goroutine de `time.AfterFunc` sin `recover`; un panic ahí tiraría Traefik.

## Deseado

1. `simpleredis` sin sentencias `select` en código de producción donde se mezclen canales por llamada o por instancia. Comportamiento observable igual: PoolTimeout → `ErrPoolWait`; cancelación/deadline del ctx despierta al que espera y devuelve el error del ctx; un over-free se descarta y se cuenta en `overFrees` con el log `simpleredis_over_free`; backoff de reintentos respeta el ctx.
2. Sin regresión de rendimiento relevante. Restricciones medidas (`zz_selectbench_test.go`, mediana de 5, ns/op, compilado / yaegi):
   - turno sin espera hoy (`select`): 23 / 176; mutex inline: 18 / 531; mutex en método (con o sin defer): ~22 / ~2300; CAS atómico: 9 / 642; recv/send simple sobre canal sin select: 22 / 81.
   - espera con pool lleno hoy (timer+select): 244 / 840; `AfterFunc`×2 + `sync.Once`: 352 / 4014; `AfterFunc`×2 sin `sync.Once`: 301 / 2697; solo timer: 175 / 1825.
   - referencia `Get` secuencial: 22 080 / 117 420.
   - Conclusión: en yaegi el coste dominante es **llamar a un método interpretado (~1 µs)**. Turno en el camino caliente: inline, sin métodos nuevos, sin `defer`; sin `sync.Once`; `context.AfterFunc` solo en la espera con pool lleno. Objetivo ≈ +0,3 % por `Get` bajo yaegi, nulo compilado. Medir antes/después (`BenchmarkGet`, `BenchmarkGetParallel` y la tabla yaegi).
3. **Cobertura de test de la versión interpretada (prioridad del desarrollador):**
   - Los repro `_Yaegi` quedan como regresión y deben pasar tras el fix; los `_Compiled` como control.
   - Guarda AST que falle si vuelve a aparecer `select` en el código de producción de `simpleredis` (como en el PR #208 del bouncer).
   - Tomar ideas de la batería de tests interpretados de https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin (tests `zzz_*`, `yaegi test` en CI, test yaegi de PR #208 con 16 poll timers, e2e con Traefik real) y de lo que ya existe en este repo (`simpleredis/yaegi_*_test.go`, `windowcounter/yaegi_flush_stop_test.go`, `reclaim/yaegi_test.go`, `e2e/`, `Test-Integration.ps1`, `.github/workflows/ci.yml`).
   - Los tests deben ser estables en CI (`go test -short` y `-race`): los `_Yaegi` se saltan con `-race` porque la carrera está dentro de `interp`.
4. PR en GitHub hacia `master`.

## Fuera de alcance / a decidir en explore

- `reclaim/table.go` `waitCtx`: impacto bajo; ¿fix aquí o follow-up?
- `zz_selectbench_*`: medición exploratoria; ¿se conserva como test opt-in (`SELECT_BENCH=1`) o se descarta?
- Otros archivos sin commit del checkout principal (`simpleredis/PRODUCTION-BUGS.md`, `bugs_production_test.go`, `zz_scratch_deferpanic_test.go`, `*/BUGS.md`) no son de esta tarea.
