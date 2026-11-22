# Explore

## Concepts

Unidades que este cambio tocaría:

| Nombre | Path | Trabajo |
| --- | --- | --- |
| `Limiter` | `windowcounter/limiter.go` | Contador sliding-window; dueño de `stopping`, `flushTimer`, `flushBusy`. |
| `Sleep` | `windowcounter/limiter.go:398-401` | `flushPending` y luego `stopFlushAndWait`. Superficie `reclaim.Hooks`. |
| `Wake` | `windowcounter/limiter.go:404-411` | Arranca timer si no `closed`, `syncRate > 0`, `flushTimer == nil`, `!stopping`. |
| `Close` | `windowcounter/limiter.go:414-424` | Marca `closed`, `flushPending`, `stopFlushAndWait`. Fuera de alcance pedir escenario Close+Sleep. |
| `stopFlushAndWait` | `windowcounter/limiter.go:457-474` | Pone `stopping=true`, despega el timer, espera el tick en vuelo. Agujero: timer `nil` pone `stopping=false` y vuelve. |
| `startFlushLocked` | `windowcounter/limiter.go:435-441` | `flushBusy.Add(1)` + `time.AfterFunc` → `flushAfterTick`. |
| `flushAfterTick` | `windowcounter/limiter.go:444-454` | `flushPending`; si `closed` / `stopping` / `flushTimer == nil` hace `Done` y no rearma; si no, otro `AfterFunc`. |
| `flushBatchEvalUnlocked` | `windowcounter/limiter.go:574-584` | Suelta `l.mu` durante el EVAL; el tick en vuelo deja `flushTimer` puesto. |
| `Table.claimDrop` | `reclaim/table.go:607-628` | Serializa el Sleep de último holder por encarnación (`slotBusy`). Enumerar, no cambiar reclaim. |
| `testFakeRedis.holdNextFlush` | `windowcounter/zzz_fake_redis_test.go:268-277` | Sujeta el EVAL de flush (evidencia black-box). |
| Contrato vivo | `openspec/specs/std_go_windowcounter_sync-flush/spec.md` | Sleep/Wake/Close: un Sleep frente a Wake; no hay dos Sleep. |
| Usage | `knowledge/devdocs/std_go_windowcounter.md` | Mismo contrato de un Sleep frente a Wake. |

Cómo se conectan:

```
  Sleep ──flushPending──┐
  Close ─flushPending───┼──► stopFlushAndWait
                        │         │
                        │    l.mu: stopping=true
                        │    timer := flushTimer; flushTimer=nil
                        │         │
                        │    ┌────┴────┐
                        │ timer==nil  timer!=nil
                        │ stopping=   Stop()? Done
                        │   false     flushBusy.Wait
                        │ return      stopping=false
                        │
  Wake ── !closed && syncRate>0 && flushTimer==nil && !stopping
       ──► startFlushLocked ── AfterFunc ──► flushAfterTick
                                                │
                                         flushPending (EVAL sin l.mu)
                                                │
                              closed/stopping/flushTimer==nil ── Done
                              else AfterFunc otra vez (flushBusy sigue a 1)

  reclaim last-holder: claimDrop ── slotBusy ── Hooks.Sleep ── parkAsleep
```

Call sites (dest HEAD `35aaa343c5185f8d5315e8715c3ceb05caed247a`; worktree HEAD `b3723e80`):

- `stopFlushAndWait`: **2** de producto (`Sleep` `:400`, `Close` `:423`) + definición `:457`. Comentario de test en `windowcounter/zzz_repro_sleep_wake_race_test.go:11`.
- `Sleep` (método `Limiter`, no `time.Sleep`): **1** definición (`:398`); **1** ejemplo de producto `README.md:37` (`reclaim.Hooks`); **0** import de `windowcounter` en `reclaim/`. Tests compilados `.Sleep()`: **14** (`zzz_repro_sleep_wake_race_test.go:39`; `zzz_repro_stale_previous_test.go:53,54`; `zzz_limiter_test.go:233,234,251,274,275,749,773`; `zzz_limiter_yaegi_test.go:200,201` en el probe `BufferedShare`; `zzz_limiter_e2e_test.go:244,245`). `TestWake_StartsTickerAfterSleep` llama dos `Sleep` **secuenciales** (timer ya nil): no cubre dos concurrentes.
- `Wake`: **1** definición (`:404`); **1** ejemplo `README.md:38`; tests `.Wake()`: **3** (`zzz_repro_sleep_wake_race_test.go:59` en bucle; `zzz_limiter_test.go:256,282`).
- `startFlushLocked`: **2** de producto (`New` `:104`, `Wake` `:410`) + definición `:435`.
- `flushAfterTick`: callback `AfterFunc` en `:440` y rearma `:452`.

Raíces buscadas: worktree `**/*.go` y `**/*.md` por `stopFlushAndWait`, `limiter.Sleep(`, `l.Sleep(`, `.Sleep()`, `limiter.Wake(`, `.Wake()`, `startFlushLocked`, `flushAfterTick`; paquete `reclaim/` por import `windowcounter` (ninguno).

Reproduce:

- Path: copia throwaway `windowcounter/zzz_throwaway_2026-11-22-windowcounter-concurrent-sleep_test.go` de `TestScale_ConcurrentSleepClearsStopping` (scale-hunt). Helpers `startTestFakeRedis` / `newSimpleRedisForTest` ya en el paquete. Fichero borrado antes del checkpoint.
- Comando: `go test -count=1 -timeout 15s -run TestScale_ConcurrentSleepClearsStopping ./windowcounter` desde worktreePath.
- Resultado: **reproduced** (falla la aserción BUG). `stopping=false` tras Sleep #2. Gist: `after Sleep#2 while Sleep#1 waits: stopping=false` / `BUG: Sleep#2 cleared stopping while Sleep#1 is still waiting; Wake can start a 2nd chain` / `--- FAIL: TestScale_ConcurrentSleepClearsStopping (0.00s)`.
- Extra black-box (throwaway, mismo fichero, borrado): EVAL sujeto con `holdNextFlush` + dos `Sleep` reales + `Wake`. **reproduced** el cuelgue de Sleep #1. Gist: `Sleep#2 returned while flush EVAL still held`; `stopping after Sleep#2 attempt: false`; `Sleep#1 still waiting after Wake`; `BUG: Sleep#1 hung after EVAL release + Wake` / `--- FAIL: ... (2.27s)`.

Hechos de fuera: in-tree `knowledge/research/ext_traefik_plugins_yaegi-select/` (select interpretado compartido); `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/` (`time.AfterFunc` / `context.AfterFunc` sí corren). Constraints Yaegi v0.16.1 ya en `requirement.md`; no se abre carpeta de research nueva.

## Decisions

Elección de diseño aplazada a propose (tech lead). Explore lista opciones; no elige el mecanismo.

Opciones candidatas (evidencia en dest):

1. **Generación / dueño de `stopping`.** Solo quien inició la parada (quien se quedó con un timer no nil, o un token de generación) puede borrar `stopping`. Sleep #2 en timer nil espera `flushBusy` sin borrar el flag. Evidencia: el early-return `:462-465` es el agujero medido (white-box `stopping=false`).
2. **Refcount de paradas.** Incrementar al entrar, decrementar al salir, `stopping=false` solo en 0; quien encuentra timer nil igual hace `Wait`. Evidencia: dos `stopFlushAndWait` concurrentes escriben el mismo `bool`.
3. **Serializar `stopFlushAndWait`.** Mutex dedicado para que Sleep #2 no entre hasta que Sleep #1 termine Wait+clear. Evidencia: el comentario `:456` ya exige no sostener `l.mu` durante Wait; un mutex extra es Yaegi-safe (sin `select`). Hueco a cubrir en propose: entre el unlock del primero y el lock del segundo, Wake podría arrancar cadena si `stopping` ya es false — Wake o el flag tienen que ver la parada pendiente.
4. **Timer nil espera sin borrar si ya hay parada en vuelo** (`stopping` ya true o `flushBusy` > 0). Parche mínimo del mismo agujero. Evidencia black-box: Sleep #2 vuelve antes de que el EVAL acabe (el caller cree que está flushed) incluso antes de Wake; hay que Wait, no solo dejar `stopping`.

Alternativas ya descartadas por evidencia:

- Reintroducir `go` + `select` en canal de stop: Constraints del requirement + spec vivo AfterFunc + research `ext_traefik_plugins_yaegi-select`.
- Dejar el early-return actual (`stopping=false` si timer nil): white-box FAIL en este dest.
- Arreglar solo en reclaim: `claimDrop` ya serializa el Sleep de último holder; reclaim no importa `windowcounter`; el trigger es varios dueños de un `Limiter`. Out of scope: no cambiar el producto reclaim.

Contrato vivo: `openspec/specs/std_go_windowcounter_sync-flush` (requisito Sleep/Wake/Close; escenario Concurrent Wake does not hang Sleep = un Sleep frente a Wake; no hay escenario de dos Sleep).

## Open questions

- Q: Qué mecanismo de parada (generación, refcount, serializar `stopFlushAndWait`, o espera en timer nil si ya hay parada) cumple el efecto pedido?
  Rank: bounded incidental — `stopFlushAndWait` tiene 2 call sites de producto enumerados (`Sleep`, `Close`) en `windowcounter/`; Out of scope «Elegir el mecanismo (contador de paradas, generación, serializar la parada)»
  Decision: assumed — propose (tech lead) chooses among the listed options
  By: explore

- Q: El cuelgue de Sleep #1 se reproduce en negro (EVAL de flush bloqueado + dos Sleep reales + Wake) sobre este dest?
  Rank: additive asked — no reshape; Unknowns de requirement.md nombra esta medición
  Decision: resolved — reproduced on dest; `holdNextFlush` + dos `Sleep` + `Wake`: Sleep #2 volvió con EVAL sujeto, `stopping=false`, Sleep #1 no volvió en 2s tras soltar el EVAL
  By: explore

- Q: Close concurrente con Sleep cae por el mismo agujero en runtime?
  Rank: bounded incidental — Close es el otro call site de `stopFlushAndWait` (1 de 2 enumerados); Out of scope «Pedir un arreglo de Close concurrente con Sleep»
  Decision: assumed — no añadir escenarios Close+Sleep a este ticket; comparten `stopFlushAndWait`, así que un helper seguro para dos Sleep sigue teniendo a Close como caller, sin cambiar el producto Close
  By: explore

- Q: Cómo serializa reclaim las transiciones de cada clave en dest (el informe dice que el trigger por reclaim es poco probable)?
  Rank: additive asked — no se cambia reclaim; Unknowns nombra este hecho
  Decision: resolved — `claimDrop` (`reclaim/table.go:607-628`) pone `slotBusy` y reclama el Sleep solo si `holders == 0` y `slotAwake`; un segundo drop no lanza otro `Hooks.Sleep` a la vez. Dos `Limiter.Sleep` concurrentes vía reclaim en una clave no salen de esa agenda. El trigger de producto es varios dueños de un `Limiter` (README `reclaim.Hooks{ Sleep: limiter.Sleep }`)
  By: explore

- Q: Coste y comportamiento Yaegi del arreglo?
  Rank: additive asked — Constraints exigen Yaegi-safe y AfterFunc; Unknowns nombra medir el arreglo
  Decision: assumed — se mantiene el modelo AfterFunc y no hay `select` interpretado en el paquete (requirement + `ext_traefik_plugins_yaegi-select` / `ext_traefik_plugins_yaegi-afterfunc`); medir compilado y Yaegi en implement cuando propose elija mecanismo. Explore no inventa una suite Yaegi para un diseño no elegido
  By: explore
