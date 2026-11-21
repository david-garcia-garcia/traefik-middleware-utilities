## Motivation
El middleware que limita peticiones con `tokenbucket` (reloj de Traefik RateLimit, en memoria o en Redis) y con `windowcounter` (ventana deslizante de Redis) responde 429 cuando el limitador deniega, pero no puede decirle al cliente cuándo reintentar ni cuánto cupo le queda. `tokenbucket.Memory.Allow` / `Redis.Allow` solo devolvían `(allowed, wait)` y `windowcounter.Take` / `Peek` / `Allow` solo `(allowed, estimated)`.

Los datos existían dentro de cada llamada y se tiraban: los tokens que quedan en el cubo (`consumeOne` los guarda en memoria; el script Lua los devuelve como tercer campo y Go no lo leía), los recuentos `current` y `previous` por separado y el inicio de la ventana fija. Sin ellos el middleware no puede rellenar `Retry-After` ni `X-RateLimit-Limit` / `-Remaining` / `-Period` / `-Reset`, y además el `wait` de `tokenbucket` se desbordaba a una duración negativa con esperas enormes (`-2562047h47m16.854775808s`), así que tampoco servía tal cual como `Retry-After`.

Sin el cambio, los clientes reciben 429 sin indicación de cuándo reintentar, y el middleware no puede ofrecer las cabeceras `X-RateLimit-*` que Traefik Hub documenta para su limitador distribuido.

Priority: P2 — dolor real para quien opera el middleware y sus clientes (429 sin `Retry-After` ni cabeceras de cupo), sin daño de datos ni contrato público roto hoy.

## Implementation
Cambio rompedor pedido por el solicitante: las firmas existentes devuelven `(Status, error)` y no hay métodos paralelos. Cada paquete tiene su `Status` en `status.go` con los mismos nombres para los campos comunes (`Allowed`, `Limit`, `Remaining`, `Period`, `Reset`, `RetryAfter`), y un único `newStatus` que construye todos los campos a partir de lo que la llamada ya tenía; los errores devuelven `Status{}`.

En `tokenbucket`, `Memory` y `Redis` pasan por el mismo `newStatus(clock, tokens, waitMicro)`: `Limit = burst`, `Remaining = floor(tokens)` acotado a `[0, burst]`, `Period = burst / rate` precalculado al construir, `Reset` = tiempo hasta cubo lleno, y la espera se reparte en `Delay` (admitido con espera, Traefik duerme) o `RetryAfter` (denegado). Todas las duraciones saturan en el máximo en vez de desbordar. `Redis.Allow` lee ahora el tercer campo del `EVAL` con la misma regla de número finito y devuelve `tokenbucket: eval tokens is not a number` si no lo es; el script Lua no cambia.

En `windowcounter`, `slidingWindow` lleva la lectura de reloj, la longitud y el inicio de la ventana, y el peso pasa a `weightAt`, compartido por la admisión y la búsqueda de reintento. `Reset` es el tiempo hasta el fin de la ventana fija alineada al epoch (decisión del solicitante). `RetryAfter`, solo al denegar, es el primer segundo entero en el que un Take más sin otro tráfico sería admitido: forma cerrada en la ventana actual o en la siguiente, corregida paso a paso con el mismo predicado de admisión; con `limit < 1` es `Reset`. En modo buffered todos los campos salen de la vista de esta instancia y no se añade ningún comando Redis.

La revisión de código dejó `newStatus` de ambos paquetes calculando los campos en variables locales y devolviendo un único literal, lo que recorta el coste compilado del `Status` en la ruta caliente sin cambiar ningún valor.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** `tokenbucket.Memory.Allow` / `Redis.Allow` y `windowcounter.Limiter.Take` / `Peek` / `Allow` devuelven `(Status, error)` en lugar de `(bool, time.Duration, error)` / `(bool, float64, error)`; la antigua espera es `Delay + RetryAfter`, el antiguo estimado es `Estimated`, y `Redis.Allow` puede devolver el nuevo error `tokenbucket: eval tokens is not a number`.
**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: P2 — dolor real para quien opera el middleware y sus clientes (429 sin `Retry-After` ni cabeceras de cupo), sin daño de datos ni contrato público roto hoy.
Reviewed head: 3538ce6c
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Not ready |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-21-ratelimit-status pushed | `git` |
| OpenSpec | ratelimit-status | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/106 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/openspec/changes/ratelimit-status/proposal.md) — modified
- [std_go_tokenbucket_status](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/openspec/changes/ratelimit-status/proposal.md) — added
- [std_go_windowcounter_sliding-take](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/openspec/changes/ratelimit-status/proposal.md) — modified
- [std_go_windowcounter_status](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/openspec/changes/ratelimit-status/proposal.md) — added


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-21-ratelimit-status on branch 2026-11-21-ratelimit-status targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/106; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué valores tiene `tokenbucket.Status` (Limit, Remaining, Period, Reset)? | additive asked — campos de un tipo nuevo; Desired 3 pide Remaining, Limit, Period y Reset. | assumed — `Limit = burst`; `Remaining = floor(max(tokens_after, 0))`; `Period = burst / rate` (derivado: tiempo en rellenar el bucket entero; documentar que no es un `period` de configuración); `Reset = max(burst − tokens_after, 0) / rate` (tiempo hasta bucket lleno). `tokens_after` es el valor que queda tras este Allow, ya reembolsado si se denegó (lo que persiste el store). Así `Limit/Period = rate` y `Remaining ≤ Limit` siempre. Implement: `Period` (precalculado por limitador) y `Reset` se calculan como `tokens × 1e9 / rate` ns (multiplicar antes de dividir), no `tokens / limitPerMicro`: medido, `5 / (10/1e6)` da 499999.99999999994 µs → 499.999999ms, y el escenario del spec exige 500ms exactos. Misma magnitud, sin cambio de decisión. | implement |
| Con `maxDelay > 0`, ¿qué valen `RetryAfter`, `Remaining` y la espera en una admisión diferida y en una denegación con refund? | additive asked — campo nuevo `Delay` en un tipo nuevo; Desired 2 y 3. | assumed — admitido (`waitMicro <= maxDelay`): `Allowed = true`, `Delay = wait`, `RetryAfter = 0`. Denegado: `Allowed = false`, `Delay = 0`, `RetryAfter = wait`. `Remaining` y `Reset` usan los tokens tras el consume (negativos en admisión diferida → `Remaining = 0`; reembolsados en denegación, que pueden ser negativos por deuda — sonda: `-2`). La espera bruta de hoy es `Delay + RetryAfter`; no se pierde información y el llamador mantiene el comportamiento de hoy (dormir `Delay`). | explore |
| Al denegar, ¿`RetryAfter` es la espera completa (`wait`) o el primer instante en que se admitiría (`wait − maxDelay`)? | additive asked — valor de un campo nuevo; Desired 2. | assumed — `RetryAfter = wait`. Es el `delay` sobre el que Traefik ya emite `Retry-After` en su propio limitador (`knowledge/research/ext_traefik_ratelimiter_token-bucket/.sources/rate-limiter-go.md`) y, reintentando entonces, el token ya está disponible sin espera. `wait − maxDelay` sería el mínimo; se documenta como diferencia y no se expone. | explore |
| ¿Basta el tercer campo de la respuesta Lua (`tostring(tokens)`) en Redis y Dragonfly, o hay que tocar el script? | additive incidental — se lee un campo que la respuesta ya trae; ningún criterio nombra el parseo de `values[2]` (es medio para Desired 3). | assumed — basta; no se toca `allowScript`. Medido en Redis 7 y Dragonfly v1.40.2 (tabla en Concepts): siempre decimal finito parseable con `strconv.ParseFloat`. | explore |
| ¿Qué hace `Redis.Allow` si el campo `tokens` no es un número finito (`nan`, `inf`, basura)? | additive incidental — ruta de error nueva para una respuesta malformada; ningún criterio la nombra. | assumed — error nuevo `tokenbucket: eval tokens is not a number` (`errEvalTokens`), con la misma regla que `errEvalWait`: nunca `Status` con `err == nil`. Hoy ese campo se ignora; el único productor de respuestas a medida es el fake de tests (`setEvalReply`), cuyos casos existentes ponen `"0"` en el tercer campo. | explore |
| ¿Paridad memoria/Redis de `Remaining` y `Reset`, dado que Redis redondea `tokens` a 14 dígitos significativos? | additive incidental — amplía un escenario de paridad existente a campos nuevos; ningún criterio lo nombra. | assumed — paridad exacta de `Allowed`, `Limit`, `Period`, `Delay`/`RetryAfter` por clase (como hoy); `Remaining` y `Reset` iguales salvo cuando el redondeo `%.14g` de Redis cruza un entero (fracción ≥ 0.99999999999995). Los escenarios de paridad usan secuencias con tokens enteros o fracciones lejos de ese borde; se documenta la salvedad. | explore |
| ¿Qué fórmula da `RetryAfter` en `windowcounter`? | additive asked — campo nuevo; Desired 2. | assumed — forma cerrada: tiempo desde ahora hasta el primer segundo entero en que un `Take` sería admitido sin más tráfico. Con `C` = current (incluye este hit si fue `Take`), `P` = previous, `W` = ventana en s, `L` = limit: si `C + 1 ≤ L` y `P > 0`, `e = ceil(W × (1 − (L − C − 1)/P))`; si `e < W` el reintento es `windowStart + e`. Si no, en la ventana siguiente (previous = `C`, current = 0): `e = ceil(W × (1 − (L − 1)/C))` acotado a `[0, W]`, reintento en `windowEnd + e`. El resultado se verifica con el mismo predicado y la misma función de peso que la admisión (±1 s por redondeo de float). `RetryAfter = time.Unix(reintento, 0).Sub(now)`; 0 cuando `Allowed`. Documentar que cada `Take` denegado intermedio también incrementa y aleja el reintento. | explore |
| ¿Qué `RetryAfter` devuelve `windowcounter` cuando `limit < 1`, si ningún `Take` se admite nunca? | additive asked — valor límite de un campo nuevo; Desired 2. | assumed — `RetryAfter = Reset` (fin de la ventana actual): valor finito y no nulo, para que el cliente no reintente en bucle. Se documenta que con `limit < 1` no hay admisión posible. `limit 0 = ilimitado` (Hub) sigue fuera de alcance; hoy `limit` no se valida y eso no cambia. | explore |
| ¿Qué valen `Remaining`, `Limit` y `Period` en `windowcounter`, en `Take` y en `Peek`? | additive asked — campos de un tipo nuevo; Desired 3 y Desired 4 (nombra `Peek`). | assumed — `Limit = limit`; `Period = window`; `Remaining = floor(max(limit − estimated, 0))` con la misma `estimated` que cada método ya calcula (Take: tras el hit; Peek: ocupación sin hit). En ambos significa "cuántos `Take` más caben ahora": Peek con `estimated = limit − 2.3` da 2. `RetryAfter = 0` cuando `Allowed`, también en Peek (su `Allowed` sigue siendo ocupación ≤ limit; en ocupación exactamente igual a `limit`, Peek da `Allowed = true`, `Remaining = 0`, `RetryAfter = 0` y el siguiente Take deniega, como hoy). `Estimated` se mantiene. | explore |
| ¿Qué precisión tiene `Reset` en `windowcounter` (reloj en segundos enteros)? | additive asked — campo nuevo; Desired 5 fija la semántica. | assumed — la semántica la resolvió el humano (Desired 5); lo asumido es la precisión: `Reset = time.Unix(windowStart + W, 0).Sub(now)`, con el `now` del mismo `slidingAt`: en `(0, W]`, con fracción de segundo. No se redondea; el formato es del middleware. Al fin de la ventana el peso de la anterior no es cero, así que `Reset` no implica cupo completo (Tension registrada en requirement.md; no es error). | explore |
| En modo buffered (`sync_rate > 0`), ¿son aceptables valores locales (`redisKnown + localDelta`) y, en caída de Redis, del cupo de esta instancia? | additive asked — campos nuevos con la misma fuente que `estimated`; Desired 3. | assumed — sí. Todos los campos salen de la misma vista que ya decide `Allowed` en esa ruta; documentarlo junto al contrato buffered existente (per-node cap con `err == nil`). No se añade lectura a Redis para mejorar las cabeceras. | explore |
| ¿Cómo se evita que una duración derivada de float desborde a negativo (`burst/rate` grande, `rate` minúsculo)? | additive incidental — conversión en campos nuevos; ningún criterio la nombra. | assumed — un helper por paquete convierte float a `time.Duration` saturando al máximo (y 0 para valores ≤ 0); lo usan `Period`, `Reset`, `RetryAfter` y `Delay`. Medido: hoy `time.Duration(1e21)` da `-2562047h…` en amd64. No cambia la admisión (`waitMicro <= maxDelay` sigue en float). | explore |
| ¿Qué pasa con `_Avoid_: remaining quota` en `## Language` de `std_go_tokenbucket.md` y `std_go_windowcounter.md` (Take, Peek, Window counter, Token bucket)? | bounded asked — cambia términos existentes de dos packets enumerados; Affected nombra `knowledge/devdocs/std_go_tokenbucket.md` y `std_go_windowcounter.md`. | assumed — explore no escribe Language. Devdocs impact define `Status` y `Remaining` ("hits/tokens que aún caben ahora, tras este Take/Allow; Peek: sin hit") y retira `remaining quota` de `_Avoid_` donde contradiga el campo nuevo, manteniendo `_Avoid_` de "Peek como reserva" y de "Peek como Allow". | explore |
| ¿Qué contratos vivos cambian? | bounded asked — dos specs vivas enumeradas (de cuatro revisadas: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`, `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`), migradas en este cambio; Affected nombra `std_go_tokenbucket_allow` y `std_go_windowcounter_sliding-take`. | assumed — propose cambia el reparto (FindSpecHost: la semántica de los campos supera el umbral de un fold de 1–3 requisitos y la regla de ≤500 caracteres obliga a partirla). Fold en `std_go_tokenbucket_allow` (MODIFIED "Allow maps admit, wait, and deny" → devuelve `Status`; ADDED lectura del campo `tokens` y `errEvalTokens`) y en `std_go_windowcounter_sliding-take` (MODIFIED Take, sliding estimate, Peek → devuelven `Status`; `Estimated` no es `Remaining`). Hojas nuevas `std_go_tokenbucket_status` y `std_go_windowcounter_status` para el significado de cada campo, saturación, paridad memoria/Redis, retry-after y modo buffered. `std_go_tokenbucket_lua-eval` y `std_go_windowcounter_sync-flush` sin delta: no tienen frase sobre el tipo de retorno (lua-eval habla de "allowed" y "clase de wait", que siguen siendo ciertos con `Delay + RetryAfter` = wait). Sin desviación. | propose |

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_standards.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_spec.md) — 1 total, 0 pending, 1 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_performance.md) — 2 total, 0 pending, 2 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-ratelimit-status/devstate/2026/11/2026-11-21-ratelimit-status/codereview_coverage.md) — 1 total, 0 pending, 1 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 3538ce6ce873dea6b5acacd2ea4d4b099be74348 | Card must match the branch you measured |

### Stored data model
None.
