# Explore

## Concepts

Un `Limiter` buffered (`syncRate > 0`) guarda un `windowState` por clave Redis `{opaqueKey}:{windowStart}` en `l.windows`. El tick de flush, `Sleep` y `Close` recorren ese mapa entero dos veces con `l.mu` cogido, aunque no haya nada que enviar. Take/Peek buffered del mismo `Limiter` esperan ese mutex. La biblioteca no reconstruye dirección de cliente, usuario, tenant ni Host: `Take`/`Peek` reciben una clave opaca del caller (`limiter.go:171`, `:187`; Language en `knowledge/devdocs/std_go_windowcounter.md`).

### Units

| Name | Path | Job |
|---|---|---|
| `Limiter.windows` | `windowcounter/limiter.go:63` | Mapa sin tope de ventanas vivas. `New` (`:97-107`) lo deja vacío. |
| `windowState` | `windowcounter/limiter.go:74-84` | `redisKnown`, `localDelta`, `expireAt`, `flushDelta`, `previousGetAt`, `unread`. |
| `slidingAt` | `windowcounter/limiter.go:132-156` | `expireAt = windowStart + 2*windowSec` (`ttlSec := 2 * windowSec`). Un mismo `Limiter` admite `window` distintos por llamada. |
| `takeBuffered` / `peekBuffered` | `windowcounter/limiter.go:227-235`, `:251-258` | Incremento/lectura bajo `l.mu`. Esperan el scan del flush. |
| `flushPending` | `windowcounter/limiter.go:477-482` | Toma `l.mu` y llama a `flushPendingLocked`. |
| `flushPendingLocked` | `windowcounter/limiter.go:494-530` | Recogida O(K) (`:498-507`), EVAL por lotes sin mutex, poda O(K) (`:520-524`). |
| `flushBatchEvalUnlocked` | `windowcounter/limiter.go:572-584` | Suelta `l.mu` durante el EVAL (contrato vivo: EVAL no sujeta el mutex). |
| `flushAfterTick` | `windowcounter/limiter.go:443-454` | Flush y luego rearma `time.AfterFunc`. Duty cycle ≈ `scan / (scan + syncRate)`. |
| `Sleep` / `Close` | `windowcounter/limiter.go:397-401`, `:414-424` | Mismo `flushPending` (Close llama en `:422`). |
| `tokenbucket.Memory` expiry list | `tokenbucket/memory.go:9-35`, `:105-177` | Lista de expiración con punteros concretos (no `container/list`). Referencia #105, no dueño de este cambio. |
| Probe e2e | `e2e/ratelimitprobe/plugin.go:152-157` | Un `New` + `Take` por request; clave de `X-Probe-Key`, no de `RemoteAddr`. |

```
Take/Peek buffered ──l.mu──► l.windows[key]
                              ▲
Sleep / Close / flushAfterTick │
        │                      │
        ▼                      │
  flushPending (Lock)          │
        │                      │
        ▼                      │
  flushPendingLocked           │
     1. range l.windows collect (O(K), idle: 0 dirty)
     2. flushBatchEvalUnlocked (Unlock / EVAL / Lock)
     3. range l.windows prune   (O(K), idle: none expire if expireAt far)
        │
        ▼
  flushAfterTick rearma AfterFunc(syncRate)
```

### Call sites

Raíces buscadas: worktree `**/*.go` (paquete `windowcounter/`, `e2e/`, `tokenbucket/`) y `openspec/specs/`.

| Symbol | Production | Tests (committed) | Notes |
|---|---:|---:|---|
| `flushPendingLocked` | 1 (`flushPending` `:481`) | 0 | Única entrada al scan. |
| `flushPending` | 3 (`Sleep` `:399`, `Close` `:422`, `flushAfterTick` `:445`) | 18 | Tests: `zzz_limiter_flush_batch_test.go` 8, `zzz_limiter_flush_batch_e2e_test.go` 3, `zzz_bench_test.go` 2, `zzz_repro_stale_previous_test.go` 2, `zzz_repro_buffered_outage_test.go` 1, `zzz_limiter_test.go` 1, `zzz_bench_yaegi_test.go` 1 (`FlushForBench`). |
| `l.windows` (prod) | 14 accesos en 8 funcs de `limiter.go` | — | `peekCountLocked`, `windowLocked`, `takeBufferedPreviousLocked`, `bufferedCountLocked`, `applyGetLocked`, `flushPendingLocked` (2 `range` + `delete`), `applyFlushOutcomeLocked`, `seedUnreadWindowLocked`. Inserts: `applyGetLocked` `:363`, `seedUnreadWindowLocked` `:620`. |
| `limiter.windows` / `l.windows` (tests) | — | 6 ficheros | `zzz_limiter_flush_batch_test.go` (`windowStateForTest`), `zzz_limiter_test.go`, `zzz_bench_test.go`, `zzz_bench_yaegi_test.go` (`DirtyWindowKeysForBench` / `RedirtyWindowsForBench`), `zzz_repro_stale_previous_test.go`. |
| `takeBuffered` / `peekBuffered` `Lock` | 2 | via Take/Peek | Toda admisión buffered del mismo `Limiter` espera el scan. |
| `windowcounter.New` fuera del paquete | 1 (`e2e/ratelimitprobe/plugin.go:152`) | Yaegi probes en `zzz_limiter_yaegi_test.go` / `zzz_bench_yaegi_test.go` | |

`BenchmarkFlush` (`zzz_bench_test.go:125`) mide 1000 claves **sucias**, no el scan idle vs K.

### Reproduce

Reproduced. Copia temporal de `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\windowcounter\zzz_scale_hunt_test.go` en `windowcounter/zzz_scale_hunt_test.go` (borrada antes del checkpoint). `go test -count=1 -timeout 20m -run 'TestScale_FlushTickScanCost|TestScale_YaegiFlushTickScanCost' -v .` desde `windowcounter/`. `TestScale_ConcurrentSleepClearsStopping` no se ejecutó.

Ambos tests **PASS** (solo `t.Logf`, sin umbral). El fallo reclamado es el coste O(K), no una aserción roja.

| Live windows | Spec compiled (`1f5ec454`) | Medido compiled (`35aaa343`) | Spec Yaegi | Medido Yaegi |
|---:|---:|---:|---:|---:|
| 10,000 | 0.2 ms | 104 µs | 1.2 ms | 1.7599 ms |
| 50,000 | — | — | 8.6 ms | 9.478766 ms |
| 200,000 | 11.3 ms | 11.123 ms | 55.7 ms | 51.946566 ms |
| 1,000,000 | 65.6 ms | 67.137 ms | — | — |

Orden de magnitud alineado con la tabla del requirement. A `syncRate` 20 ms y 200k ventanas: Yaegi ~72 % del tiempo con mutex ocupado (`51.95/(51.95+20)`); compilado ~36 % (`11.12/(11.12+20)`). Tick idle = dos `range` con `localDelta==0` y `expireAt` a 1 h, sin EVAL.

### Outside facts

- In-tree: `knowledge/devdocs/std_go_windowcounter.md` (Language: clave opaca; AfterFunc por Yaegi; batch 256).
- Research ya existente, sin escritura nueva: `ext_traefik_plugins_yaegi-select/` (v0.16.1 `select` compartido), `ext_traefik_plugins_yaegi-generics/` (Value `any` / method set). El pin `github.com/traefik/yaegi v0.16.1` está en `go.mod`.
- In-tree #105: `tokenbucket/memory.go` lista de expiración con punteros concretos; comentario `:32` y `:147-148` (no `container/list`; cada llamada de método bajo Yaegi cuesta microsegundos).
- Deuda ya catalogada (no tomada aquí): `knowledge/debt/2026-09-13-windowcounter-outage-windows-unbounded.md`.

## Decisions

Chosen seam: none this phase (deferred to tech-lead propose).

Opciones con evidencia (ninguna elegida ni rechazada):

1. **Conjunto sucio** — mapa o slice de claves con `localDelta > 0`. El collect de `:498-507` pasaría a O(dirty). Hay que mantenerlo en `takeBuffered` (`localDelta++`) y en `applyFlushOutcomeLocked` (cuando el delta vuelve a 0). Coste Yaegi extra por cada Take si el mantenimiento es otra llamada de método (`tokenbucket/memory.go:147-148`).
2. **Agrupación por `windowStart`** — `map[inicio]map[clave]*windowState` (scale-bug-report § 1). La poda borra un grupo entero. `slidingAt` alinea `windowStart` al epoch de **esa** `window`; un `Limiter` mezcla duraciones.
3. **Agrupación por `expireAt`** — variante in-tree de (2): `expireAt = windowStart + 2*windowSec` (`limiter.go:145-151`). Agrupa lo que caduca a la vez cuando las `window` no coinciden.
4. **Lista de expiración estilo `tokenbucket.Memory` (#105)** — `head`/`tail` con punteros concretos (`tokenbucket/memory.go`). En tokenbucket el TTL es homogéneo y cada `Allow` mueve al final. Aquí `expireAt` es por ventana y no se refresca en cada Take salvo bump si el nuevo `expireAt` es mayor (`windowLocked` `:282-284`). Una lista por recencia no ordena caducidad heterogénea; haría falta ordenar por `expireAt` o cubos por segundo.
5. **Tope de mapa** — `New` y el tipo no tienen cap (`limiter.go:63`, `:97-107`). Cubre K, no el O(K) del tick idle por debajo del tope. Requirement Out of scope: tope como política de producto distinta del coste O(sucias + que caducan ahora). Deuda previa: `knowledge/debt/2026-09-13-windowcounter-outage-windows-unbounded.md`.

Live contract: `openspec/specs/std_go_windowcounter_sync-flush/spec.md` (flush buffered/exact, EVAL sin mutex, batch ≤ 1024, no perder `local_delta`, Sleep/Wake/Close). El catálogo no exige el scan O(K); un cambio de índice no puede alterar esos SHALL.

## Open questions

- Q: Qué estructura (conjunto sucio, agrupación por `windowStart` / `expireAt`, lista de expiración estilo `tokenbucket` #105, tope, u otra) cumple O(sucias + que caducan ahora) bajo Yaegi v0.16.1 sin perder deltas ni romper `std_go_windowcounter_sync-flush`?
  Rank: bounded asked — 1 call site de `flushPendingLocked` (`flushPending` `:481`) y 3 callers de producción de `flushPending` (Sleep, Close, flushAfterTick); raíces `**/*.go`; Desired «O(dirty windows + windows that expire now)» y Unknowns «Qué estructura (conjunto sucio, agrupación por `windowStart`, u otra)»
  Decision: assumed — defer to propose; options are dirty-set, windowStart grouping, expireAt grouping, tokenbucket concrete expiry list (#105), cap
  By: explore

- Q: Cuánto dura el mutex de un tick idle sobre dest `35aaa343` frente a la tabla del spec (`1f5ec454`)?
  Rank: additive asked — medición, no reshape; Unknowns «Tiempo de mutex de un tick idle re-medido sobre dest `35aaa343`»
  Decision: resolved — compiled 10k=104µs, 200k=11.123ms, 1M=67.137ms; Yaegi 10k=1.7599ms, 50k=9.478766ms, 200k=51.946566ms (`TestScale_FlushTickScanCost`, `TestScale_YaegiFlushTickScanCost`). Misma magnitud que el spec.
  By: explore

- Q: Cuál es el radio de impacto de cambiar `flushPendingLocked` frente a Sleep/Close/Take/Peek y a los `zzz_*`?
  Rank: bounded asked — call sites enumerados; raíces worktree `**/*.go`; Unknowns «Radio de impacto de cambiar `flushPendingLocked`»
  Decision: resolved — producción: 1 callee (`flushPendingLocked`), 3 callers de `flushPending`, 14 accesos a `l.windows` en `limiter.go`, Take/Peek buffered esperan `l.mu`; tests: 18 llamadas a `flushPending`, 6 ficheros leen el mapa. Semántica de flush anclada en `std_go_windowcounter_sync-flush`.
  By: explore

- Q: Cómo afirmar el hold time del mutex en un test de regresión sin tautología y sin un probe de caja blanca que choque con Yaegi?
  Rank: additive asked — test nuevo de este cambio; Desired «A regression test/benchmark proves that the mutex hold time of an idle tick does not grow with the number of live windows» y Unknowns «Cómo afirmar el hold time»
  Decision: assumed — defer to propose; options are compiled+Yaegi wall time of idle `flushPending` (EVAL vacío ⇒ wall ≈ hold), Take latency under the ticker, or a test-only hook. `BenchmarkFlush` mide 1000 dirty, no idle vs K. Helpers con sufijo del slug del ticket (Constraints). No copiar `ScanProbe`/`itoaWC` del lote scale-hunt.
  By: explore

- Q: Un tope de `l.windows` hace falta para el coste pedido, o es otro producto?
  Rank: bounded incidental — inserts de producción enumerados (`applyGetLocked` `:363`, `seedUnreadWindowLocked` `:620`); raíces `**/*.go`; Out of scope «Un tope de ventanas como política de producto distinta del coste O(sucias + que caducan ahora)»; Desired no nombra un cap
  Decision: assumed — defer to propose; cap stays an option for the cost; Out of scope as a distinct product policy; existing debt `knowledge/debt/2026-09-13-windowcounter-outage-windows-unbounded.md`
  By: explore
