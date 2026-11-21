# Explore

## Concepts

### Unidades que toca el cambio

| Unidad | Ruta | Trabajo hoy |
|---|---|---|
| Semáforo de turnos en uso | `simpleredis/simpleredis.go` campo `inUseTurns chan struct{}` (+ `overFrees`) | Cuenta sockets en uso; `New` lo crea lleno (`ensureInUseTurns`). |
| Toma de turno | `simpleredis/pool.go` `borrowSocket` (líneas 125–149) | `select` no bloqueante sobre `inUseTurns`; si falla, `time.NewTimer(PoolTimeout)` + `select` sobre `inUseTurns`, `ctx.Done()`, `timer.C`. |
| Devolución de turno | `simpleredis/pool.go` `freeInUseTurn` (75–87) | `select` con envío no bloqueante; `default` = over-free contado y logueado. |
| Tope de sockets vivos | `simpleredis/pool.go` `liveCap` / `ensureInUseTurns` (42–71) | `cap(inUseTurns)` tras `New`; `PoolSize()` lo expone. |
| Backoff entre reintentos | `simpleredis/commands_exec.go` `waitUntil` (137–149), 1 llamador: `exec` (`commands_exec.go:48`) | `time.NewTimer` + `defer Stop` + `select` sobre `ctx.Done()` / `timer.C`. |
| Vigilante de holders sin `Done` | `reclaim/table.go` `waitCtx` (149–172), llamador `watch` (574) | `select` sobre `finished` y `tick.C`; solo holders con `Done()==nil` (Background, la forma Yaegi). |
| Medición interpretada | `simpleredis/interpretedcost_test.go` `BenchmarkYaegiGet` (434) | Dueño ya existente del coste de `Get` bajo Yaegi. |
| Guarda AST previa | `simpleredis/simpleredis_test.go` `TestSessionSourceImports` (103–134) | Precedente `go/parser` sobre los `.go` no test del paquete. |

### Cómo encajan

```
 exec (commands_exec.go)
   ├─ waitUntil(ctx, backoff) ........ select{ctx.Done, timer.C}            ← select #3
   └─ borrowSocket(ctx)
        ├─ fast take ................. select{<-inUseTurns; default}         ← select #1 (camino caliente)
        │     └─ saturado: NewTimer + select{<-inUseTurns, ctx.Done, timer.C} ← select #2
        └─ runOnConn → do → release → freeInUseTurn
                                        select{inUseTurns<-; default→overFree} ← select #4 (camino caliente)

 Yaegi v0.16.1 interp/run.go func _select: `cases` se reserva una vez por nodo y cada
 ejecución escribe ahí sus canales antes de reflect.Select, sin lock → dos goroutines en
 la misma sentencia pueden esperar/recibir en los canales de la otra (otra llamada, otro cliente).
```

Diseño elegido (ver Decisions):

```
 SimpleRedis: inUseMu sync.Mutex · freeTurns int · turnCap int (frozen en New) · turnWaiters []*turnWaiter (FIFO)
 turnWaiter:  ready chan struct{} · granted bool

 fast take (inline):   Lock; freeTurns>0 → freeTurns--; Unlock            (sin método nuevo, sin defer)
 saturado:             Lock; append waiter; Unlock
                       child := WithTimeout(ctx, PoolTimeout); stop := context.AfterFunc(child, abandon)
                       <-w.ready  (recv simple, sin select); stop(); cancel()
                       granted → turno · ctx.Err() → ctx.Err() · si no → contextStop / errPoolWait
 free (inline):        Lock; waiters → pop[0], granted=true, close(ready) · freeTurns<turnCap → ++ · else over-free; Unlock; log fuera del lock
 abandon (AfterFunc):  Lock; si w sigue en la cola → quitar, close(ready); si ya se concedió → nada; Unlock
 waitUntil:            child := WithTimeout(ctx, delay); <-child.Done(); cancel(); ctx.Err() → ctx.Err() · si no → contextStop
```

Invariante: `freeTurns > 0 ⇒ turnWaiters vacía` (un free con espera entrega en mano), así que el fast take no adelanta a nadie y cada `close(ready)` ocurre una sola vez bajo `inUseMu` (sin `sync.Once`).

### Call sites (raíces: todo el worktree salvo `devstate/`, grep `inUseTurns` y `waitUntil(`)

- `inUseTurns` en código de producción: `simpleredis/pool.go` (liveCap, ensureInUseTurns, freeInUseTurn, borrowSocket) y `simpleredis/simpleredis.go` (campo). 2 archivos.
- `inUseTurns` en tests: `chaos_pool_test.go` (comentario), `commands_deadline_test.go:179`, `not_from_new_test.go:12-13`, `panic_safety_test.go:44,46,68`, `pool_test.go:714-718`; más la sonda `repro_yaegi_select_probe_test.go` (checkout principal). 5 archivos + 1 sonda.
- `freeInUseTurn` directo en tests: `pool_test.go:650`, `slog_test.go:127`.
- `waitUntil`: 1 llamador de producción (`commands_exec.go:48`).
- Contrato vivo que nombra "in-use-turn channel": `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (líneas 22, 229, 244, 455, 473, 496, 499, 507, 582, 602, 615, 635) y `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (672, 677). Docs: `knowledge/devdocs/std_go_simpleredis.md` (87–88), `knowledge/devdocs/std_go_test-suites.md` (47, 76).
- `select` de producción en todo el repo: `simpleredis/pool.go:79,126,131`, `simpleredis/commands_exec.go:143`, `reclaim/table.go:156,164`. Ningún otro paquete (`backendbackoff`, `iplookup`, `tokenbucket`, `traefikemulator`, `windowcounter`).

### Reproducción (worktree, HEAD `c390a37`, árbol = `origin/master` + devstate)

Comando: `go test ./simpleredis/ -run 'TestReproSelect_' -count=1 -v` con las copias de `repro_yaegi_select_*` del checkout principal (retiradas después; siguen sin commit en el checkout principal). Dos ejecuciones:

| Sonda | Compilado | Yaegi ejecución 1 | Yaegi ejecución 2 |
|---|---|---|---|
| `WaitUntil` (1 ms, ctx 300 ms) | PASS | FAIL: `shortErr` 6 / 7 623 llamadas | FAIL: 21 / 5 441 |
| `BorrowWait` (pool 1, turno retenido) | PASS | FAIL: `nilNil` 2 / 5 280 | FAIL: `nilNil` 12 / 5 312 |
| `TwoPools` (2 clientes × 16 goroutines, 0,4 s) | PASS | FAIL: `freeA` 30/32, `overFreesB` 2 | FAIL: `freeB` 31/32, `overFreesA` 1 |

`patientEarly` 0 y `hung` 0 en ambas: la espera que despierta con el canal de otra llamada y el cuelgue en `<-timer.C` **no se reprodujeron** en esta revisión.

Panic vía API pública (sonda temporal borrada; `Get` con `PoolSize` 2, 24 goroutines, ctx 1–5 ms, `PoolTimeout` 20 ms, bajo Yaegi): el proceso de test cayó en la ronda 0 a los 0,2 s con `reflect: call of reflect.Value.Field on zero Value`. Marcos Yaegi: `resp.go:19:12` (`do`) ← `pool.go:39:6` (`close`) ← `pool.go:228:5` (`release`) ← `commands_exec.go:83:17` (defer de `runOnConn`) ← `commands_exec.go:23:5` (`exec`) ← `commands.go:10:17` (`Get`). Es la misma posición `resp.go:19` que cita la especificación: es la posición que Yaegi atribuye al marco de `do`, no la del acceso a `conn`; el `nil` llega desde `borrowSocket` `(nil, nil)` y explota también en `release`→`close`. Brecha de prepare cerrada.

### Coste medido en esta máquina (línea base "antes")

`SELECT_BENCH=1 go test ./simpleredis/ -run TestSelectBenchTable` (mediana de 5, ns/op, compilado / yaegi):

| Fila | Compilado | Yaegi |
|---|---|---|
| turno sin espera: `select` (hoy) | 26 | 172 |
| turno sin espera: mutex inline | 18 | 503 |
| turno sin espera: mutex en método (defer / sin defer) | 23 / 23 | 2 272 / 2 430 |
| turno sin espera: CAS atómico | 10 | 598 |
| turno sin espera: recv/send simple | 23 | 71 |
| turno 8 goroutines: `select` / mutex en método | 39 / 134 | 329 / 1 522 |
| espera pool lleno: timer+`select` (hoy) | 240 | 842 |
| espera pool lleno: AfterFunc×2 + `sync.Once` / sin Once / solo timer | 369 / 409 / 239 | 3 304 / 2 512 / 1 790 |
| referencia: `Get` secuencial | 19 571 | 91 232 |

`go test -bench` (5 × 2 s): `BenchmarkGet` mediana 19 338 ns/op (18 589–19 693), 1 009 B/op, 26 allocs/op; `BenchmarkGetParallel` mediana 16 611 ns/op (15 927–20 453), 20 dials; `BenchmarkYaegiGet` mediana 123 681 ns/op (92 523–142 421, ruido ≈ ±20 %).

Estimación del diseño elegido en el camino caliente: Yaegi +331 ns por `Get` (503 − 172) ≈ +0,27 % de `BenchmarkYaegiGet` / +0,36 % del `Get` secuencial de la tabla; compilado −8 ns (neutro). El ruido de `BenchmarkYaegiGet` es mayor que el efecto: el "después" interpretado no puede probar +0,3 %, solo descartar una regresión grande.

### Hechos de fuera usados

- Yaegi `_select`: `$GOMODCACHE/github.com/traefik/yaegi@v0.16.1/interp/run.go` `func _select` (leído en esta fase: `cases := make([]reflect.SelectCase, nbClause+1)` fuera de `n.exec`; `n.exec` escribe `cases[i].Chan` y `cases[nbClause] = f.done` y llama `reflect.Select(cases)` sin lock). Research: `knowledge/research/ext_traefik_plugins_yaegi-select/` (escrito en esta fase).
- Símbolos stdlib de Yaegi v0.16.1 (`stdlib/go1_21_*.go` y `go1_22_*.go`): `context.AfterFunc`, `context.WithTimeout`, `context.WithoutCancel`, `time.AfterFunc`, `sync.Mutex`, `sync.Cond` presentes. `context.AfterFunc` ya probado bajo Yaegi: `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/`; usado en producción bajo Yaegi en `simpleredis/resp.go` `watchConnClose` y `reclaim/table.go:558`, `reclaim/alias.go:117`.
- Batería interpretada del bouncer (GitHub MCP, `david-garcia-garcia/crowdsec-bouncer-traefik-plugin` @ `82498271`): PR #208 añade `pkg/lapi/poll_timer.go` (`time.AfterFunc`, sin `select`) y `pkg/lapi/zzz_poll_timer_test.go` con `Test_lapiHasNoSelectStatement` (guarda `go/parser` sobre `*.go` no test del paquete) y `TestYaegi_pollTimer_manyLoopsStopIndependently` (16 poll timers de producción, para la mitad, a través del **binario** `yaegi` vía `pkg/yaegitest`, se salta si no está en PATH). CI `main.yml`: instala yaegi v0.16.1, `make yaegi_test` = `yaegi test -v .` (solo paquete raíz, cuyos `zzz_*_test.go` están escritos para ser interpretables), y job aparte `go test -race -count=1 -p 1 ./pkg/...`. E2E con Traefik real en `tests/e2e/real/Test-Integration.ps1` (Pester).
- Este repo ya ejecuta el código de producción bajo el mismo Yaegi v0.16.1 **dentro de `go test`** (dependencia de módulo `interp.New` + `writeGopathSimpleredis`): `simpleredis/yaegi_*_test.go`, `windowcounter/yaegi_flush_stop_test.go`, `reclaim/yaegi_test.go`; corren en el job `Unit` (`-short`) y, salvo los saltados por `raceDetectorOn`, en `Unit race`. E2E con Traefik real: jobs `integration*` (`Test-Integration.ps1`, plugin `e2e/simpleredisprobe`).
- Duración actual: `go test -short ./simpleredis/` (sin repro) 18,9 s; los repro añaden ≈ 15 s cuando pasan las 3 rondas (≈ 1,8+2,4+1,2 s compilado y ≈ 0,4–0,8 s por ronda yaegi). Límite del job `Unit`: 2 min por binario de test.
- `-race` no se puede ejecutar en esta máquina (sin `gcc`); solo CI lo mide.
- Deuda ajena observada: `knowledge/debt/2026-09-13-reclaim-grace-select.md` describe `waitGraceOrWake` con `select`; en este árbol ya no existe (`reclaim/table.go:651` usa `time.AfterFunc`). No se toca aquí.

## Decisions

- **Semáforo**: `inUseTurns chan struct{}` → `inUseMu sync.Mutex` + `freeTurns int` + `turnCap int` (fijado en `New`) + cola FIFO `turnWaiters []*turnWaiter`; dueño `simpleredis/pool.go` (+ campos en `simpleredis/simpleredis.go`).
- **Fast take y free**: inline en `borrowSocket` y en el cuerpo actual de `freeInUseTurn` (método ya existente, no uno nuevo), `Lock`/`Unlock` explícitos, sin `defer`, sin `sync.Once`; el log `simpleredis_over_free` y `overFrees.Add` se mantienen, el log fuera del lock.
- **Espera saturada**: waiter en cola + `context.WithTimeout(ctx, PoolTimeout)` + un `context.AfterFunc` que lo retira bajo `inUseMu`; el waiter espera con recepción simple `<-w.ready`. Clasificación igual que hoy: concedido → sigue; `ctx.Err()` → se devuelve sin log; si no → `contextStop(ctx)` o log `pool_wait` + `errPoolWait`. Es el único sitio con `AfterFunc` nuevo.
- **`waitUntil`**: `context.WithTimeout(ctx, delay)` + `<-child.Done()` + `cancel()`; `ctx.Err()` → `ctx.Err()`, si no `contextStop(ctx)`. Misma forma que la espera saturada (simetría: "ctx hijo con plazo, despertar por su `Done`").
- **"No viene de New"**: `turnCap == 0` sustituye a `inUseTurns == nil` en `borrowSocket`, `freeInUseTurn` y `liveCap`; `PoolSize()` / `OverFrees()` / `PoolTimeout()` sin cambio de API.
- **`Close`** sigue sin despertar a los waiters (hoy tampoco; esperan `PoolTimeout` o su ctx). Sin cambio de comportamiento.
- **Guarda AST**: `TestSessionSourceHasNoSelect` junto a `TestSessionSourceImports` en `simpleredis/simpleredis_test.go` (mismo recorrido `go/parser` sobre los `.go` no test), falla con `archivo:línea` de cada `*ast.SelectStmt`. Solo `simpleredis` (lo pedido).
- **Regresión interpretada**: los tres pares `TestReproSelect_*` pasan a ser permanentes (`simpleredis/repro_yaegi_select_crosstalk_test.go` + `repro_yaegi_select_probe_test.go`, convención `repro_*` ya usada en `reclaim/` y `windowcounter/`); la sonda se adapta al nuevo semáforo (hoy lee/retiene `sr.inUseTurns`). `_Compiled` corren en `-short` y en `-race`; `_Yaegi` corren en `-short` y se saltan con `raceDetectorOn`.
- **Tests compilados del semáforo** (dueño `simpleredis/pool_test.go`): FIFO sin turno perdido ni doble cuando la concesión y el abandono por timeout/ctx compiten; cancelación del ctx despierta al waiter; `PoolTimeout` → `ErrPoolWait`; over-free contado; reposo `free == cap`. Corren también bajo `-race`, que es donde el mutex nuevo se vigila.
- **Tests que leen `inUseTurns`** (5 archivos): migran a un lector de test con nombre de test (p. ej. `inUseTurnsFreeForTest`) que lee `freeTurns`/`turnCap` bajo `inUseMu`.
- **Medición antes/después**: `BenchmarkGet`, `BenchmarkGetParallel` (compilado) y `BenchmarkYaegiGet` (`interpretedcost_test.go`, dueño existente), mediana de 5; la línea base está arriba.
- **`zz_selectbench_*`**: se descartan (no entran en el cambio). La tabla de primitivas queda registrada aquí y en `requirement.md`.
- **`reclaim/table.go` `waitCtx`**: follow-up, no se arregla aquí → `knowledge/debt/2026-11-21-reclaim-waitctx-select.md` (fila en `issues.md`).
- **CI**: sin job nuevo; ni paso `yaegi test` ni caso Pester nuevo. Los tests interpretados nuevos corren en los jobs `Unit` y `Unit race` existentes.

Rechazadas:

- Canal conservado y protegido por mutex (`len(ch)` + recv/send simple bajo lock): mantiene literal el término "in-use-turn channel" de las specs y los 5 tests, pero deja un canal que ya no sincroniza nada y añade ≈ 70 ns Yaegi por turno.
- CAS atómico para el fast take: 598 ns Yaegi (peor que el mutex inline) y la espera saturada seguiría necesitando un mutex y una cola.
- Semáforo como tipo con métodos `tryTake`/`give`: ≈ 2 300 ns Yaegi por turno (llamada a método interpretado ≈ 1 µs).
- Espera con `time.AfterFunc` + `context.AfterFunc` + `sync.Once`: 3 304 ns Yaegi; `sync.Once` sobra porque el cierre ya ocurre una vez bajo `inUseMu`. Queda como alternativa si `WithTimeout`+`AfterFunc` mide peor en implement.
- Solo timer (sin `context.AfterFunc`): la cancelación del ctx no despertaría al waiter (requisito 1).
- `time.Sleep(delay)` en `waitUntil`: no respeta la cancelación del ctx durante el backoff.
- Paso `yaegi test` en CI (como el bouncer): los tests de este repo importan `github.com/traefik/yaegi/interp` y crean su propio intérprete; el arnés en proceso ya ejecuta el código de producción bajo Yaegi v0.16.1 dentro del job `Unit` sin binario extra.
- Guarda AST en todos los paquetes de producción ahora: `reclaim/table.go` la rompería; se propone junto al follow-up de `reclaim`.

Contrato vivo: `std_go_simpleredis_tcp-session` — MODIFIED en los requisitos que nombran el "in-use-turn channel" (New records settings; Full pool wait returns redis:unreachable — "stop the timer"; Client not from New; Peer-closed idle vintage; Extra in-use turn return MUST NOT hang; Chaos pool; Close cycles; Panic between borrow and release; Panic inside idle keep-or-close) y ADDED: el código de sesión no contiene `select` y la concurrencia interpretada (dos clientes, espera saturada, backoff) no cruza turnos ni plazos. `std_go_simpleredis_resp-commands` — MODIFIED en el requisito de MSetEX concurrente (línea 672) por el mismo término. `std_go_ci_test-suites`: sin cambio (no hay job nuevo).

## Open questions

- Q: ¿Qué sustituye al canal `inUseTurns` y a sus dos `select` (fast take, espera saturada, free) sin perder ni duplicar turnos y sin regresión en el camino caliente bajo Yaegi?
  Rank: bounded asked — cambia la unidad existente `inUseTurns` con call sites enumerados (2 archivos de producción, 5 de test + 1 sonda, 12+2 líneas de spec; raíces: todo el worktree salvo devstate) y los migra aquí; requirement.md «Deseado» 1 y 2 y Unknowns «Diseño concreto del sustituto de cada select»
  Decision: resolved — mutex + contador + `turnCap` + cola FIFO de waiters; fast take y free inline sin método nuevo ni `defer`; la espera saturada usa un único cierre de abandono armado con `time.AfterFunc(PoolTimeout)` + `context.AfterFunc(ctx)`, sin `sync.Once`, y recepción simple. Implement midió que `WithTimeout`+`AfterFunc` es peor (compilado 609–684 frente a 331–373 ns por espera; Yaegi 4 523–4 742 frente a 4 131–4 633 ns) y aplicó la alternativa prevista. Pruebas: `TestTurn*` en `pool_test.go`, y `TestReproSelect_*_Yaegi` con `-count=20` 80/80.
  By: implement
- Q: ¿Cómo espera `waitUntil` el backoff sin `select` respetando el ctx?
  Rank: bounded asked — cambia el cuerpo de `waitUntil` con 1 llamador enumerado (`commands_exec.go:48`, raíz `./`); requirement.md «Deseado» 1 (backoff de reintentos respeta el ctx)
  Decision: assumed — `context.WithTimeout(ctx, delay)` + `<-child.Done()`; `ctx.Err()` → `ctx.Err()`, si no `contextStop(ctx)`.
  By: explore
- Q: ¿Soporta Yaegi v0.16.1 todos los símbolos stdlib del diseño?
  Rank: additive asked — no cambia estructura; requirement.md Unknowns «Si context.AfterFunc (Go 1.21) se expone y se comporta bien bajo yaegi v0.16.1»
  Decision: resolved — `context.AfterFunc`, `context.WithTimeout`, `time.AfterFunc`, `sync.Mutex` están en `stdlib/go1_21_*` y `go1_22_*`; `context.AfterFunc` ya corre bajo Yaegi en `simpleredis/resp.go` y `reclaim/table.go` (`knowledge/research/ext_traefik_plugins_yaegi-afterfunc/`). CI fija Go 1.21, que tiene `context.AfterFunc`.
  By: explore
- Q: ¿Qué alcance tiene la guarda AST contra `select`?
  Rank: additive asked — test nuevo; requirement.md «Deseado» 3 («Guarda AST que falle si vuelve a aparecer select en el código de producción de simpleredis»)
  Decision: assumed — solo `simpleredis`, en `simpleredis/simpleredis_test.go` junto a `TestSessionSourceImports`. Extenderla a `reclaim` y al resto de paquetes va en el follow-up de `reclaim`.
  By: explore
- Q: ¿Cómo quedan los repro como regresión permanente y con qué presupuesto de estabilidad en CI (`-short`, `-race`)?
  Rank: additive asked — tests nuevos; requirement.md «Deseado» 3 (los `_Yaegi` quedan como regresión, `_Compiled` como control, estables en `-short` y `-race`, `_Yaegi` saltados con `-race`)
  Decision: assumed — permanentes con 3 rondas; `_Yaegi` saltados con `raceDetectorOn`, `_Compiled` también bajo `-race`; tras el fix implement ejecuta los `_Yaegi` con `-count=20` en local sin un solo fallo antes de commitear; si un umbral de tiempo (`shortSlow` > 100 ms, `patientEarly` < 700 ms, `longEarly` < 550 ms) falla por carga, se ensancha el umbral, no se quita la comprobación. La detección determinista de la regresión es la guarda AST; los repro son la prueba de comportamiento. Propose añade: un cuarto par `TestReproSelect_SaturatedGet_*` (Get público saturado, PoolSize 2, 24 goroutines, ctx 1–5 ms: cubre concesión/abandono/timeout y el panic que tumbó el proceso, que `BorrowWait` no ejercita porque retiene el único turno todo el rato), un contador `cancelLate` en `BorrowWait` (despertar por cancel vía `context.AfterFunc`), helpers de turno definidos en el propio probe (el paquete interpretado solo ve lo copiado) y probes sin `select` propio. La guarda AST lleva un caso con fuente en memoria para no pasar en vacío.
  By: propose
- Q: ¿Se adopta del bouncer un paso `yaegi test` en CI o un caso e2e nuevo con Traefik real?
  Rank: additive incidental — no se reestructura nada; requirement.md Out of scope («Paso yaegi test nuevo en CI y e2e con Traefik real»)
  Decision: assumed — no. El arnés en proceso (`interp.New` + GOPATH temporal) ya ejecuta producción bajo Yaegi v0.16.1 en `Unit`; `yaegi test` tendría que interpretar tests que importan `interp` (inferencia, no probado). Los jobs `integration-redis`/`integration-dragonfly` ya cargan `simpleredis` en Traefik real.
  By: explore
- Q: ¿Se corrige `reclaim/table.go` `waitCtx` en este cambio?
  Rank: additive incidental — la decisión no reestructura nada; requirement.md lo deja en «Fuera de alcance / a decidir en explore»
  Decision: assumed — no; follow-up `knowledge/debt/2026-11-21-reclaim-waitctx-select.md`. Solo lo alcanzan holders con `Done()==nil`, cuyo `Err()` no cambia, así que el cruce (inferencia, no reproducido) como mucho retiene una goroutine vigilante hasta que termina otra encarnación.
  By: explore
- Q: ¿Se conservan `zz_selectbench_*` como test opt-in (`SELECT_BENCH=1`)?
  Rank: additive incidental — archivos no versionados; requirement.md «Fuera de alcance / a decidir en explore»
  Decision: assumed — no. Miden réplicas sintéticas, y las filas "hoy" describen código que este cambio borra. La medición permanente es `BenchmarkGet`, `BenchmarkGetParallel` y `BenchmarkYaegiGet` (`interpretedcost_test.go`); la tabla queda en este archivo.
  By: explore
- Q: ¿Qué umbral de rendimiento acepta el cambio, si el ruido de `BenchmarkYaegiGet` (±20 %) es mayor que el efecto esperado (+0,3 %)?
  Rank: additive asked — criterio de aceptación; requirement.md «Deseado» 2 («Medir antes/después»)
  Decision: assumed — compilado: medianas de `BenchmarkGet`/`BenchmarkGetParallel` dentro de ±5 % de la línea base y 26 allocs/op sin cambio en `Get` sin contención; interpretado: mediana de `BenchmarkYaegiGet` dentro del rango de la línea base (92 523–142 421 ns/op). El sobrecoste aislado esperado (+331 ns Yaegi por turno) es el aceptado por el requirement. Implement: la carga de esta máquina variaba entre ejecuciones, así que el par compilado se comparó ejecutando antes y después uno tras otro. Resultado: `Get` +0,6 % (26 allocs/op), `GetParallel` +0,4 %. `BenchmarkYaegiGet` da 124 287 ns/op, dentro del rango. Frente a la línea base de explore, la mediana aislada de `GetParallel` (18 768) queda un 13 % por encima. Las cifras están en `tasks.md` § Verification notes.
  By: implement
- Q: ¿Se quita el salto con `-race` de `TestYaegiErrorpath_PoolWait` cuando ya no haya `select`?
  Rank: additive incidental — ningún criterio lo nombra
  Decision: assumed — no se toca: la pila de carrera documentada también incluye `itype.refType` dentro de `interp`, y `-race` no se puede medir en esta máquina (sin gcc).
  By: explore
