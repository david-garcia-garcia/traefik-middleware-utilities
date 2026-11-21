## Motivation
Not yet.

## Implementation
Not yet.

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 3219046f
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
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/106 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-21-ratelimit-status on branch 2026-11-21-ratelimit-status targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/106; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué valores tiene `tokenbucket.Status` (Limit, Remaining, Period, Reset)? | additive asked — campos de un tipo nuevo; Desired 3 pide Remaining, Limit, Period y Reset. | assumed — `Limit = burst`; `Remaining = floor(max(tokens_after, 0))`; `Period = burst / rate` (derivado: tiempo en rellenar el bucket entero; documentar que no es un `period` de configuración); `Reset = max(burst − tokens_after, 0) / rate` (tiempo hasta bucket lleno). `tokens_after` es el valor que queda tras este Allow, ya reembolsado si se denegó (lo que persiste el store). Así `Limit/Period = rate` y `Remaining ≤ Limit` siempre. | explore |
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
| ¿Qué contratos vivos cambian? | bounded asked — dos specs vivas enumeradas (de cuatro revisadas: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`, `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`), migradas en este cambio; Affected nombra `std_go_tokenbucket_allow` y `std_go_windowcounter_sliding-take`. | assumed — MODIFIED en `std_go_tokenbucket_allow` (incluye la lectura del campo `tokens`, el error `errEvalTokens` y la paridad de campos nuevos) y `std_go_windowcounter_sliding-take`. `std_go_tokenbucket_lua-eval` y `std_go_windowcounter_sync-flush` sin delta salvo que propose encuentre una frase sobre el tipo de retorno; si la hay, se registra como desviación. | explore |

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 3219046f830657cbf068e4e1da561426fd9995a3 | Card must match the branch you measured |

### Stored data model
None.