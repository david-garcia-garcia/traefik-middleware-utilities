# Explore

## Concepts

### Unidades que toca el cambio

| Unidad | Ruta | Trabajo hoy |
|---|---|---|
| `tokenbucket.Memory.Allow` | `tokenbucket/memory.go:65` | Consume 1 token en memoria; devuelve `(bool, time.Duration, error)`. Calcula `tokens` y `waitMicro` con `consumeOne` y descarta `tokens`. |
| `tokenbucket.Redis.Allow` | `tokenbucket/redis.go:52` | Mismo consume vía `EVAL` de `allowScript`; exige 3 campos, lee `values[1]` (wait) y descarta `values[2]` (tokens). |
| `consumeOne` / `clockConfig` | `tokenbucket/clock.go:21`, `:56` | Fórmulas Lua de Traefik (refill, cap a `burst`, consume, refund si `wait > maxDelay`). Solo conoce `rate`, `burst`, `maxDelay`, `ttl`. No cambia. |
| `allowScript` | `tokenbucket/lua.go` | Devuelve `{"true", tostring(wait_duration), tostring(tokens)}`. No cambia (ver la pregunta del tercer campo Lua). |
| `windowcounter.Limiter.Take` / `Peek` / `Allow` | `windowcounter/limiter.go:157`, `:173`, `:188` | Devuelven `(allowed bool, estimated float64, err error)`; `Allow` es alias de `Take`. |
| `slidingAt` / `slidingWindow` | `windowcounter/limiter.go:119`, `:128` | Lee el reloj una vez, calcula claves, `weight` (segundos enteros), `ttlSec`, `expireAt`. No expone `now` ni el fin de ventana. |
| `takeExact` / `takeBuffered` / `peekExact` / `peekBuffered` | `windowcounter/limiter.go:193`, `:214`, `:227`, `:241` | Tienen `current` y `previous` por separado y devuelven solo `current + previous × weight`. |

No hay reconstrucción de identidad: la clave sigue siendo opaca y del llamador (`openspec/specs/std_go_tokenbucket_allow/spec.md` "Caller owns the key", `std_go_windowcounter_sliding-take` igual). No aplica la pregunta de dueño de identidad.

### Cómo se conectan

```
middleware (fuera del repo; escribe 429 y cabeceras)
   │  Allow(ctx,key)                         │  Take/Peek/Allow(ctx,key,limit,window)
   ▼                                         ▼
tokenbucket                              windowcounter
 ├─ Memory.Allow ─┐                       ├─ slidingAt ── now, windowStart, weight
 │                ├─► consumeOne          │      │
 └─ Redis.Allow ──┘   (Go)  ◄── misma     ├─ takeExact / peekExact   (Redis: EVAL INCR, GET)
        │               fórmula ──►       └─ takeBuffered / peekBuffered (memoria + flush)
        ▼                                         │
   allowScript (Lua)                              ▼
   → wait, tokens                         current, previous, weight
        │                                         │
        ▼                                         ▼
   [nuevo] tokenbucket.Status             [nuevo] windowcounter.Status
   Allowed Limit Remaining Period         Allowed Limit Remaining Period
   Reset RetryAfter Delay                 Reset RetryAfter Estimated
```

Seams: (1) el retorno público de cada método; (2) dentro de `tokenbucket`, el punto donde Memory y Redis ya tienen `tokens` y `waitMicro`; (3) dentro de `windowcounter`, el punto donde cada ruta ya tiene `current`, `previous` y el `slidingWindow` del mismo instante.

### Call sites (enumerados)

Raíces buscadas: todo el worktree, `*.go` con `\.(Allow|Take|Peek)\(` y con imports de `tokenbucket"` / `windowcounter"`; `*.md`, `*.ps1`, `*.yml` con el mismo patrón, excluyendo `devstate/` y `openspec/changes/archive/`.

- Importadores Go fuera de los dos paquetes: **0** (`not found`).
- `tokenbucket`: **78** líneas en **16** ficheros `_test.go` (incluye fuentes interpretadas en `limiter_yaegi_test.go`, `memory_yaegi_test.go`, `bench_yaegi_test.go`, y `bench_test.go` / `bench_e2e_test.go`).
- `windowcounter`: **110** líneas en **15** ficheros: 14 `_test.go` (incluye `limiter_yaegi_test.go`, `bench_yaegi_test.go`) + 1 en `limiter.go` (alias `Allow` → `Take`).
- Documentación: `README.md` líneas 76, 84 (`Peek`/`Take`) y 100 (`tokenbucket` `Allow`); la 198 es `backendbackoff` (fuera de alcance). `knowledge/devdocs/std_go_tokenbucket.md` y `std_go_windowcounter.md` (snippets y `## Language`).
- Llamadores externos (el middleware en otro repo): **no enumerables** desde este repo.

### Reproducción

- Afirmación del ticket: las librerías no devuelven datos para `Retry-After` / `X-RateLimit-*`. Confirmado por lectura: firmas en `tokenbucket/memory.go:65`, `tokenbucket/redis.go:52`, `windowcounter/limiter.go:157/173/188`; `values[2]` se descarta en `redis.go` (solo se parsea `values[1]`).
- Línea base ejecutada en este worktree (HEAD `8a338a68`):
  - `go test -short -count=1 ./tokenbucket/ ./windowcounter/` → ambos `ok` (incluye `TestYaegi_*`: AllowBurst, AllowRefund, MemoryCapAndExpiry, TakeUntilDeny, PeekThenTake, BufferedShareSleepDoesNotHang, MethodFlushStopDoesNotHang).
  - Live con contenedores desechables `redis:7-alpine` y `dragonfly:v1.40.2` (mismas imágenes que `.github/workflows/ci.yml`): `TestLive_Limiter`, `TestYaegiLive_RedisAndDragonfly`, `TestLive_FlushBatch*` → `PASS` en ambos paquetes y ambos motores.
  - Observación no relacionada: los tests live de `windowcounter` usan claves y reloj fijos; una segunda ejecución sobre el mismo motor sin `FLUSHALL` falla (`take 1 denied`, `estimated 3 want 1`). CI usa contenedores nuevos, así que no rompe CI. Se anota aquí y no se toca (Bound the ask).
- Sonda del tercer campo Lua (`tokens`) ejecutando el `allowScript` real con `redis-cli --eval` en ambos motores:

| Caso | Redis 7 | Dragonfly v1.40.2 |
|---|---|---|
| burst 5, rate 1/s, 6.º consume denegado (maxDelay 0) | `true 1000000 0` | `true 1000000.0 0.0` |
| rate 3/s, tokens fraccionarios | `0.3`, `0.6`, `0.9` | `0.3`, `0.6`, `0.9` |
| admitido con espera (maxDelay 2s, burst 1) | `true 1000000 -1` | `true 1000000.0 -1.0` |
| denegado con refund tras deuda | `true 3000000 -2` | `true 3000000.0 -2.0` |
| burst 1e15 | `1e+15` | `1e+15` |
| burst 123456789012345678 | `1.2345678901235e+17` | `123456789012345677` |

  Conclusión: el campo es siempre un número decimal finito que `strconv.ParseFloat` acepta (`"4"`, `"4.0"`, `"-1.0"`, `"1e+15"`). Redis formatea con 14 dígitos significativos (Lua 5.1 `%.14g`); Dragonfly (Lua 5.4) añade `.0` a enteros. En denegación el campo ya es el valor reembolsado y puede ser negativo (deuda de admisiones con espera).
- Overflow de duración: `go run` de prueba (scratch, fuera del árbol) con `time.Duration(1e18 µs × 1000)` y con `burst/rate = 1e12/0.001 s` da `-2562047h47m16.854775808s` en `go1.25.6 windows/amd64`. El `wait` actual ya tiene este overflow (el test `allow_wait_duration_overflow_test.go` solo comprueba `allowed`); cualquier campo de duración nuevo lo heredaría.
- Yaegi y retorno de struct por valor: ya existe en código interpretado (`slidingAt` devuelve `slidingWindow` por valor, `windowcounter/limiter.go:128`, cubierto por `TestYaegi_TakeUntilDeny`).

### Hechos externos usados

- In-tree: `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (Traefik responde 429 + `Retry-After` cuando `delay > maxDelay`; extracto `.sources/rate-limiter-go.md`, sin el formato exacto del valor) y `knowledge/research/ext_traefik_hub_distributed-rate-limit/` (nombres de cabeceras; "tokens left in the bucket after the reservation"; sin formatos ni `Retry-After`).
- Medido en esta ejecución: formato del tercer campo Lua en Redis 7 y Dragonfly v1.40.2 (tabla anterior). No se escribe carpeta de research nueva: la medida está aquí y la decisión no depende de documentación del proveedor.
- Devdocs consumidos: `knowledge/devdocs/index.md` → `index_std_go.md` → `std_go_tokenbucket.md`, `std_go_windowcounter.md`. Ambos tienen `_Avoid_: remaining quota` en `## Language` (ver la pregunta de `## Language`).

## Decisions

### Elegido

- API: `tokenbucket` `Memory.Allow` / `Redis.Allow` y `windowcounter` `Take` / `Peek` / `Allow` devuelven `(Status, error)`; sin métodos paralelos (decisión humana, Desired 4).
- Un tipo `Status` por paquete, en su propio fichero (`tokenbucket/status.go`, `windowcounter/status.go`), stdlib solo; mismos nombres para los campos comunes: `Allowed bool`, `Limit int64`, `Remaining int64`, `Period time.Duration`, `Reset time.Duration`, `RetryAfter time.Duration`. Extras: `tokenbucket.Status.Delay` (espera de una admisión diferida) y `windowcounter.Status.Estimated` (estimación deslizante de hoy).
- `tokenbucket`: un único constructor de `Status` a partir de (`clockConfig`, `tokens` tras el consume, `waitMicro`) que usan Memory y Redis por igual (simetría; hoy el mapeo `allowed`/`wait` está duplicado en `memory.go` y `redis.go`). `consumeOne` y la regla de admisión no cambian.
- `tokenbucket` Redis: leer `values[2]` con la misma validación de número finito que `values[1]`; `allowScript` no cambia.
- `windowcounter`: `slidingWindow` lleva también el instante leído y el fin de la ventana fija, para que `Reset` y `RetryAfter` usen el mismo reloj que fijó `weight`. Las cuatro rutas (`takeExact`, `takeBuffered`, `peekExact`, `peekBuffered`) pasan `current` y `previous` por separado a un único constructor de `Status`; la condición de admisión (`estimated <= limit`) no cambia.
- `windowcounter`: el peso por segundo se calcula con una sola función compartida entre `slidingAt` y el cálculo de `RetryAfter`, para que el predicado de reintento sea bit a bit el mismo que el de admisión.
- Duraciones derivadas de floats (`RetryAfter`, `Reset`, `Period`, `Delay`) se saturan a la duración máxima en lugar de desbordar a negativo.
- Rendimiento: medir antes (dest `ee9f5c11`) y después en la misma máquina, compilado y Yaegi, con los benchmarks de #105; actualizar `README.md` `## Rate limiters compared` si cambian las cifras.

### Rechazado

- Métodos paralelos o wrappers (`AllowStatus`, etc.): el humano los descartó (Out of scope).
- Devolver más valores sueltos en la tupla: el humano eligió un struct.
- Un `Status` compartido en un paquete común: crea una dependencia nueva entre paquetes hoy independientes y otra copia GOPATH en cada test Yaegi.
- Cambiar `allowScript` para devolver más campos o formatear con `string.format("%.17g")`: el tercer campo ya basta (sonda en ambos motores); tocar el script cambia su digest y el contrato `std_go_tokenbucket_lua-eval` sin necesidad.
- Pedir `average`/`period` al llamador para `Limit`/`Period` de `tokenbucket`: superficie de configuración nueva que nadie pidió; la librería solo conoce `rate` y `burst`.
- `RetryAfter` de `windowcounter` por iteración segundo a segundo: O(ventana) por denegación, caro bajo Yaegi; se usa forma cerrada con verificación del predicado.
- `Reset` de `windowcounter` truncado a segundos enteros: el fin de ventana ya es un segundo exacto; truncar el `now` sobrestima hasta 1 s. El formato de la cabecera es del middleware (Out of scope).
- Escribir cabeceras o el 429 desde la librería (Desired 1).
- Semántica Hub (`denyOnError`, `limit 0 = ilimitado`, `sourceCriterion`): sigue fuera (decisión humana).

### Contrato vivo

- `openspec/specs/std_go_tokenbucket_allow/spec.md` — MODIFIED: "Allow maps admit, wait, and deny" describe `allowed` + wait; pasa a `Status`.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — sin delta: el script no cambia y su paridad (línea 51: `allowed` y clase de wait) sigue siendo cierta. La paridad de los campos nuevos y la lectura del campo `tokens` se escriben en `std_go_tokenbucket_allow` (spec nombrada en Affected).
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — MODIFIED: retornos de Take/Peek; el escenario "Usage is the sliding estimate … it is not remaining quota" se reescribe (Estimated se mantiene; Remaining es un campo aparte).
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — probablemente sin delta (sus escenarios hablan de `allowed` y estimación, que siguen existiendo como campos); propose lo confirma.

## Open questions

- Q: ¿Cambiamos las firmas públicas existentes (`Memory.Allow`, `Redis.Allow`, `Take`, `Peek`, `Allow`) a `(Status, error)`?
  Rank: structural asked — contrato con llamadores externos (el middleware) no enumerables desde este repo; dentro del repo 188 líneas en 30 ficheros + README + 2 devdocs, todas migrables aquí; Desired 4 nombra este cambio de firma.
  Decision: resolved — decisión humana (Desired 4): cambio rompedor, sin métodos paralelos. Se migran todos los call sites del repo (tests compilados, fuentes Yaegi, benchmarks, README, devdocs).
  By: explore

- Q: ¿Qué valores tiene `tokenbucket.Status` (Limit, Remaining, Period, Reset)?
  Rank: additive asked — campos de un tipo nuevo; Desired 3 pide Remaining, Limit, Period y Reset.
  Decision: assumed — `Limit = burst`; `Remaining = floor(max(tokens_after, 0))`; `Period = burst / rate` (derivado: tiempo en rellenar el bucket entero; documentar que no es un `period` de configuración); `Reset = max(burst − tokens_after, 0) / rate` (tiempo hasta bucket lleno). `tokens_after` es el valor que queda tras este Allow, ya reembolsado si se denegó (lo que persiste el store). Así `Limit/Period = rate` y `Remaining ≤ Limit` siempre.
  By: explore

- Q: Con `maxDelay > 0`, ¿qué valen `RetryAfter`, `Remaining` y la espera en una admisión diferida y en una denegación con refund?
  Rank: additive asked — campo nuevo `Delay` en un tipo nuevo; Desired 2 y 3.
  Decision: assumed — admitido (`waitMicro <= maxDelay`): `Allowed = true`, `Delay = wait`, `RetryAfter = 0`. Denegado: `Allowed = false`, `Delay = 0`, `RetryAfter = wait`. `Remaining` y `Reset` usan los tokens tras el consume (negativos en admisión diferida → `Remaining = 0`; reembolsados en denegación, que pueden ser negativos por deuda — sonda: `-2`). La espera bruta de hoy es `Delay + RetryAfter`; no se pierde información y el llamador mantiene el comportamiento de hoy (dormir `Delay`).
  By: explore

- Q: Al denegar, ¿`RetryAfter` es la espera completa (`wait`) o el primer instante en que se admitiría (`wait − maxDelay`)?
  Rank: additive asked — valor de un campo nuevo; Desired 2.
  Decision: assumed — `RetryAfter = wait`. Es el `delay` sobre el que Traefik ya emite `Retry-After` en su propio limitador (`knowledge/research/ext_traefik_ratelimiter_token-bucket/.sources/rate-limiter-go.md`) y, reintentando entonces, el token ya está disponible sin espera. `wait − maxDelay` sería el mínimo; se documenta como diferencia y no se expone.
  By: explore

- Q: ¿Basta el tercer campo de la respuesta Lua (`tostring(tokens)`) en Redis y Dragonfly, o hay que tocar el script?
  Rank: additive incidental — se lee un campo que la respuesta ya trae; ningún criterio nombra el parseo de `values[2]` (es medio para Desired 3).
  Decision: assumed — basta; no se toca `allowScript`. Medido en Redis 7 y Dragonfly v1.40.2 (tabla en Concepts): siempre decimal finito parseable con `strconv.ParseFloat`.
  By: explore

- Q: ¿Qué hace `Redis.Allow` si el campo `tokens` no es un número finito (`nan`, `inf`, basura)?
  Rank: additive incidental — ruta de error nueva para una respuesta malformada; ningún criterio la nombra.
  Decision: assumed — error nuevo `tokenbucket: eval tokens is not a number` (`errEvalTokens`), con la misma regla que `errEvalWait`: nunca `Status` con `err == nil`. Hoy ese campo se ignora; el único productor de respuestas a medida es el fake de tests (`setEvalReply`), cuyos casos existentes ponen `"0"` en el tercer campo.
  By: explore

- Q: ¿Paridad memoria/Redis de `Remaining` y `Reset`, dado que Redis redondea `tokens` a 14 dígitos significativos?
  Rank: additive incidental — amplía un escenario de paridad existente a campos nuevos; ningún criterio lo nombra.
  Decision: assumed — paridad exacta de `Allowed`, `Limit`, `Period`, `Delay`/`RetryAfter` por clase (como hoy); `Remaining` y `Reset` iguales salvo cuando el redondeo `%.14g` de Redis cruza un entero (fracción ≥ 0.99999999999995). Los escenarios de paridad usan secuencias con tokens enteros o fracciones lejos de ese borde; se documenta la salvedad.
  By: explore

- Q: ¿Qué fórmula da `RetryAfter` en `windowcounter`?
  Rank: additive asked — campo nuevo; Desired 2.
  Decision: assumed — forma cerrada: tiempo desde ahora hasta el primer segundo entero en que un `Take` sería admitido sin más tráfico. Con `C` = current (incluye este hit si fue `Take`), `P` = previous, `W` = ventana en s, `L` = limit: si `C + 1 ≤ L` y `P > 0`, `e = ceil(W × (1 − (L − C − 1)/P))`; si `e < W` el reintento es `windowStart + e`. Si no, en la ventana siguiente (previous = `C`, current = 0): `e = ceil(W × (1 − (L − 1)/C))` acotado a `[0, W]`, reintento en `windowEnd + e`. El resultado se verifica con el mismo predicado y la misma función de peso que la admisión (±1 s por redondeo de float). `RetryAfter = time.Unix(reintento, 0).Sub(now)`; 0 cuando `Allowed`. Documentar que cada `Take` denegado intermedio también incrementa y aleja el reintento.
  By: explore

- Q: ¿Qué `RetryAfter` devuelve `windowcounter` cuando `limit < 1`, si ningún `Take` se admite nunca?
  Rank: additive asked — valor límite de un campo nuevo; Desired 2.
  Decision: assumed — `RetryAfter = Reset` (fin de la ventana actual): valor finito y no nulo, para que el cliente no reintente en bucle. Se documenta que con `limit < 1` no hay admisión posible. `limit 0 = ilimitado` (Hub) sigue fuera de alcance; hoy `limit` no se valida y eso no cambia.
  By: explore

- Q: ¿Qué valen `Remaining`, `Limit` y `Period` en `windowcounter`, en `Take` y en `Peek`?
  Rank: additive asked — campos de un tipo nuevo; Desired 3 y Desired 4 (nombra `Peek`).
  Decision: assumed — `Limit = limit`; `Period = window`; `Remaining = floor(max(limit − estimated, 0))` con la misma `estimated` que cada método ya calcula (Take: tras el hit; Peek: ocupación sin hit). En ambos significa "cuántos `Take` más caben ahora": Peek con `estimated = limit − 2.3` da 2. `RetryAfter = 0` cuando `Allowed`, también en Peek (su `Allowed` sigue siendo ocupación ≤ limit; en ocupación exactamente igual a `limit`, Peek da `Allowed = true`, `Remaining = 0`, `RetryAfter = 0` y el siguiente Take deniega, como hoy). `Estimated` se mantiene.
  By: explore

- Q: ¿Qué precisión tiene `Reset` en `windowcounter` (reloj en segundos enteros)?
  Rank: additive asked — campo nuevo; Desired 5 fija la semántica.
  Decision: assumed — la semántica la resolvió el humano (Desired 5); lo asumido es la precisión: `Reset = time.Unix(windowStart + W, 0).Sub(now)`, con el `now` del mismo `slidingAt`: en `(0, W]`, con fracción de segundo. No se redondea; el formato es del middleware. Al fin de la ventana el peso de la anterior no es cero, así que `Reset` no implica cupo completo (Tension registrada en requirement.md; no es error).
  By: explore

- Q: En modo buffered (`sync_rate > 0`), ¿son aceptables valores locales (`redisKnown + localDelta`) y, en caída de Redis, del cupo de esta instancia?
  Rank: additive asked — campos nuevos con la misma fuente que `estimated`; Desired 3.
  Decision: assumed — sí. Todos los campos salen de la misma vista que ya decide `Allowed` en esa ruta; documentarlo junto al contrato buffered existente (per-node cap con `err == nil`). No se añade lectura a Redis para mejorar las cabeceras.
  By: explore

- Q: ¿Cómo se evita que una duración derivada de float desborde a negativo (`burst/rate` grande, `rate` minúsculo)?
  Rank: additive incidental — conversión en campos nuevos; ningún criterio la nombra.
  Decision: assumed — un helper por paquete convierte float a `time.Duration` saturando al máximo (y 0 para valores ≤ 0); lo usan `Period`, `Reset`, `RetryAfter` y `Delay`. Medido: hoy `time.Duration(1e21)` da `-2562047h…` en amd64. No cambia la admisión (`waitMicro <= maxDelay` sigue en float).
  By: explore

- Q: ¿Cuánto cuesta el struct de retorno compilado y bajo Yaegi?
  Rank: additive incidental — solo medida; ningún criterio de Desired la nombra (sale de Unknowns y de la memoria del proyecto).
  Decision: resolved — decisión humana: medir compilado y Yaegi. Implement corre `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/` (incluye `BenchmarkYaegi*`) en dest `ee9f5c11` y en la rama, en la misma máquina y sesión; reporta ns/op, allocs/op y `redis-cmds/op` (debe quedar igual) y actualiza `README.md` `## Rate limiters compared` si cambian. Los campos se calculan en cada llamada (aritmética O(1)); `RetryAfter` de `windowcounter` solo se calcula al denegar.
  By: explore

- Q: ¿Qué pasa con `_Avoid_: remaining quota` en `## Language` de `std_go_tokenbucket.md` y `std_go_windowcounter.md` (Take, Peek, Window counter, Token bucket)?
  Rank: bounded asked — cambia términos existentes de dos packets enumerados; Affected nombra `knowledge/devdocs/std_go_tokenbucket.md` y `std_go_windowcounter.md`.
  Decision: assumed — explore no escribe Language. Devdocs impact define `Status` y `Remaining` ("hits/tokens que aún caben ahora, tras este Take/Allow; Peek: sin hit") y retira `remaining quota` de `_Avoid_` donde contradiga el campo nuevo, manteniendo `_Avoid_` de "Peek como reserva" y de "Peek como Allow".
  By: explore

- Q: ¿Qué contratos vivos cambian?
  Rank: bounded asked — dos specs vivas enumeradas (de cuatro revisadas: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`, `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`), migradas en este cambio; Affected nombra `std_go_tokenbucket_allow` y `std_go_windowcounter_sliding-take`.
  Decision: assumed — propose cambia el reparto (FindSpecHost: la semántica de los campos supera el umbral de un fold de 1–3 requisitos y la regla de ≤500 caracteres obliga a partirla). Fold en `std_go_tokenbucket_allow` (MODIFIED "Allow maps admit, wait, and deny" → devuelve `Status`; ADDED lectura del campo `tokens` y `errEvalTokens`) y en `std_go_windowcounter_sliding-take` (MODIFIED Take, sliding estimate, Peek → devuelven `Status`; `Estimated` no es `Remaining`). Hojas nuevas `std_go_tokenbucket_status` y `std_go_windowcounter_status` para el significado de cada campo, saturación, paridad memoria/Redis, retry-after y modo buffered. `std_go_tokenbucket_lua-eval` y `std_go_windowcounter_sync-flush` sin delta: no tienen frase sobre el tipo de retorno (lua-eval habla de "allowed" y "clase de wait", que siguen siendo ciertos con `Delay + RetryAfter` = wait). Sin desviación.
  By: propose
