# 2026-11-21-ratelimit-status

issueHost: local · issueRef: none · source: caller spec (pedido del humano en chat, 2026-11-21)

## Pedido del humano

1. Hoy ninguno de los dos rate limiters (`tokenbucket/` y `windowcounter/`) escribe cabeceras HTTP (son librerías; el middleware escribe el 429). Eso se mantiene: la librería sigue sin escribir HTTP.
2. El humano necesita que **ambos** limitadores devuelvan la información para que el middleware pueda poner `Retry-After`.
3. Y también (ambos) la información que permita poner: `X-RateLimit-Remaining`, `X-RateLimit-Limit`, `X-RateLimit-Period`, `X-RateLimit-Reset`.
4. Decisión del humano (API): **cambiar las firmas existentes** — `tokenbucket` `Memory.Allow`/`Redis.Allow` y `windowcounter` `Take`/`Peek`/`Allow` pasan a devolver `(Status, error)` (un struct con los datos). Cambio rompedor aceptado explícitamente; no añadir métodos paralelos.
5. Decisión del humano (semántica): en `windowcounter`, `X-RateLimit-Reset` = tiempo hasta el **fin de la ventana fija actual** (alineada al epoch), no hasta cupo completo.

## Análisis previo en chat (hipótesis del asistente, NO confirmadas por el humano)

- tokenbucket: el Lua de Redis ya devuelve `tokens` como 3er campo (hoy se descarta); `Memory` tiene `entry.tokens`. Mapeo propuesto: Limit=burst, Remaining=floor(max(tokens,0)), Period=burst/rate (inferencia: la librería solo recibe rate, no el average/period de Traefik), Reset=(burst−tokens)/rate (tiempo a bucket lleno), RetryAfter=wait cuando deniega.
- windowcounter: `estimated = current + previous×weight`; solo se devuelve la suma. Take cuenta el hit aunque deniegue. RetryAfter calculable en forma cerrada desde current/previous/weight/limit. Modo buffered: valores locales (redisKnown+localDelta). Remaining=floor(max(limit−estimated,0)), Limit=limit, Period=window.
- Perf: la memoria del proyecto exige benchmarks en compilado Y Yaegi para cambios con impacto de rendimiento.
- Nota: el análisis en chat se hizo sobre un master local 11 commits por detrás; origin/master ya incluye #105 (perf ratelimit: O(1) tokenbucket admission, batched windowcounter flush) y #54/#60/#62/#63.
