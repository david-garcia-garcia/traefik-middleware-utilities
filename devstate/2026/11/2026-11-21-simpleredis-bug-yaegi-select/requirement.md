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

---

<!-- Secciones añadidas por opd-prepare (worktree en origin/master 185c4f6). Lo anterior es la especificación del llamante, copiada íntegra. -->

## Current (code)

- Yaegi fijado en v0.16.1 — `go.mod` (línea `require github.com/traefik/yaegi v0.16.1`).
- `_select` de yaegi reserva `cases := make([]reflect.SelectCase, nbClause+1)` una vez por nodo `select`, en la compilación del nodo y no por ejecución — `$GOMODCACHE/github.com/traefik/yaegi@v0.16.1/interp/run.go` (`func _select`, línea 3749). La sobrescritura sin lock en cada ejecución la afirma la especificación; aquí solo se ha comprobado la reserva única.
- `freeInUseTurn`: `select` con envío no bloqueante a `sr.inUseTurns` y `default` que suma `overFrees` y registra `simpleredis_over_free` — `simpleredis/pool.go` (líneas 75–87).
- `borrowSocket`: `select` no bloqueante sobre `<-sr.inUseTurns`; si no hay turno, `time.NewTimer(sr.inUseTurnWait())` y `select` sobre `sr.inUseTurns`, `ctx.Done()` y `timer.C` (con `timer.Stop()` + `<-timer.C` en las dos primeras ramas); PoolTimeout → log `pool_wait` + `errPoolWait` — `simpleredis/pool.go` (líneas 112–151).
- `ErrPoolWait` / `errPoolWait` / `IsPoolWait` — `simpleredis/simpleredis.go` (líneas 34–56); contador `overFrees` y su lector — `simpleredis/simpleredis.go` (líneas 83–84, 153).
- `waitUntil`: `time.NewTimer(delay)` + `defer timer.Stop()` + `select` sobre `ctx.Done()` y `timer.C` — `simpleredis/commands_exec.go` (líneas 137–149).
- No hay otras sentencias `select` en código de producción de `simpleredis` (solo las tres anteriores) — `simpleredis/pool.go`, `simpleredis/commands_exec.go`.
- `reclaim` `waitCtx`: `time.NewTicker(20ms)`, `select` no bloqueante sobre `finished` y `select` bloqueante sobre `finished` / `tick.C`; según su comentario solo lo alcanzan holders con `Done()` nil — `reclaim/table.go` (líneas 145–172).
- `windowcounter` usa `time.AfterFunc` en `startFlushLocked` / `flushAfterTick`, con comentario sobre `interp._select`; `flushAfterTick` no tiene `recover` — `windowcounter/limiter.go` (líneas 369–397).
- Archivos de reproducción `repro_yaegi_select_probe_test.go` y `repro_yaegi_select_crosstalk_test.go` — `not found` en el worktree; existen sin commit en `D:\repositories\traefik-middleware-utilities\simpleredis\`.
- Medición `zz_selectbench_test.go` / `zz_selectbench_probe_test.go` — `not found` en el worktree; existen sin commit en el checkout principal (`simpleredis\`).
- `BenchmarkGet` — `simpleredis/bench_test.go` (línea 35); `BenchmarkGetParallel` — `simpleredis/bench_test.go` (línea 257).
- Tabla de coste interpretado (yaegi) existente — `simpleredis/interpretedcost_test.go`.
- Arnés yaegi de `simpleredis` (`interp.New` sobre un GOPATH temporal con el paquete copiado: `writeGopathSimpleredis`, `evalClientprobe`) — `simpleredis/yaegi_test.go` (líneas 239–304); otros tests yaegi — `simpleredis/yaegi_defer_test.go`, `simpleredis/yaegi_errorpath_test.go`, `windowcounter/yaegi_flush_stop_test.go`, `windowcounter/limiter_yaegi_test.go`, `reclaim/yaegi_test.go`.
- Salto de tests yaegi con `-race` vía constante `raceDetectorOn` — `simpleredis/racedetector_on_test.go`, `simpleredis/racedetector_off_test.go` (usado p. ej. en `simpleredis/yaegi_errorpath_test.go` línea 191 y `reclaim/yaegi_test.go`).
- Guarda AST sobre el código del paquete: existe un precedente con `go/parser` (solo imports, `TestSessionSourceImports`) — `simpleredis/simpleredis_test.go` (líneas 103–134); guarda AST contra `select` — `not found`.
- CI: job unit `go test -short -timeout 2m -count=1 -v ./...` y job race `go test -race -short -timeout 10m -count=1 -v ./...` — `.github/workflows/ci.yml` (líneas 38, 51); paso `yaegi test` en CI — `not found`.
- E2E yaegi con Pester — `Test-Integration.ps1`, `scripts/integration-tests.simpleredis.Tests.ps1`, `scripts/integration-tests.reclaim.Tests.ps1`; plugin sonda — `e2e/simpleredisprobe/plugin.go`.
- Entrada del panic citado (`runOnConn` → `do` con `conn == nil`): `do` empieza en `simpleredis/resp.go` línea 18; la línea 19 del worktree es `contextStop(ctx)`, no un acceso a `conn`.

## Out of scope

- `windowcounter` `flushAfterTick` sin `recover` (inferencia no reproducida de la especificación; no está en «Deseado»).
- Archivos sin commit del checkout principal ajenos a la tarea: `simpleredis/PRODUCTION-BUGS.md`, `simpleredis/bugs_production_test.go`, `simpleredis/zz_scratch_deferpanic_test.go`, `*/BUGS.md`.
- Sentencias `select` en código de test (la guarda pedida es sobre código de producción).
- Actualizar yaegi o parchear `interp` (no pedido; la especificación toma v0.16.1 como dado).
- Paquetes distintos de `simpleredis` salvo lo que explore decida para `reclaim/table.go` `waitCtx` (la especificación lo deja abierto).
- Paso `yaegi test` nuevo en CI y e2e con Traefik real: la especificación los cita como fuente de ideas («tomar ideas»), no como entregable obligatorio.

## Unknowns

- Diseño concreto del sustituto de cada `select` (mutex inline, CAS atómico, `context.AfterFunc`, recv/send simple) y su equivalencia exacta con el comportamiento observable (PoolTimeout → `ErrPoolWait`, ctx → error del ctx, over-free contado y logueado, backoff que respeta el ctx). Explore.
- Coste real antes/después bajo yaegi y compilado (`BenchmarkGet`, `BenchmarkGetParallel`, tabla yaegi); las cifras de la especificación son de una medición exploratoria no versionada.
- Si `context.AfterFunc` (Go 1.21) se expone y se comporta bien bajo yaegi v0.16.1 en el camino de espera con pool lleno; CI fija `go-version: "1.21"` (`.github/workflows/ci.yml`).
- Estabilidad y duración de los repro `_Yaegi` dentro del límite de 2 min del job unit `-short`; antes del fix uno de ellos tumbó el proceso de test.
- Cuelgue en `<-timer.C` tras `Stop()` con tick robado: no reproducido.
- `reclaim/table.go` `waitCtx`: fix en este cambio o follow-up (decisión de explore).
- `zz_selectbench_*`: conservar como opt-in (`SELECT_BENCH=1`) o descartar (decisión de explore).

## Tensions

- «Sin `select`» frente a «sin regresión»: las rutas no bloqueantes (toma de turno y over-free) necesitan semántica try-send/try-recv; según la tabla de la especificación, en yaegi todas las alternativas no bloqueantes medidas (mutex inline 531, CAS 642 ns/op) son más caras que el `select` actual (176); el objetivo ≈ +0,3 % por `Get` asume ese sobrecoste.
- Alcance del punto 1 («donde se mezclen canales por llamada o por instancia») frente a la guarda AST pedida, que prohíbe cualquier `select` en producción de `simpleredis`: la guarda es más estricta que el criterio del punto 1.
- Los repro `_Yaegi` detectan la regresión de forma probabilística (fallos 1–27 por ejecución antes del fix); la detección determinista recae en la guarda AST.
- La especificación sitúa los repro en el checkout principal sin commit; el worktree no los tiene y hay que traerlos al cambio.
- La especificación cita el panic en `resp.go:19`; en el worktree esa línea es `contextStop(ctx)` dentro de `do` (`simpleredis/resp.go`), así que la línea exacta procede de otra revisión o de la traza; explore debe reproducirlo sobre el worktree.
- Commit `origin/master` del worktree (185c4f6) difiere del `HEAD` local del checkout principal (ba52347); las rutas y líneas de este archivo son las del worktree.
