# Requirement
IssueKey: 2026-11-21-ratelimit-status

Fuente: pedido del humano en chat (2026-11-21), `ticket/source.md`. Base: `origin/master` @ `ee9f5c11` (incluye #105).

## Problem

El middleware que usa estas librerías no puede rellenar `Retry-After` ni `X-RateLimit-Remaining` / `X-RateLimit-Limit` / `X-RateLimit-Period` / `X-RateLimit-Reset` porque ni `tokenbucket` ni `windowcounter` devuelven los datos necesarios. Hoy solo devuelven `(allowed, wait)` y `(allowed, estimated)`.

## Current (code)

- Las librerías no escriben HTTP; el middleware escribe el 429 — `tokenbucket/clock.go` (doc de paquete), `windowcounter/limiter.go` (doc de paquete); spec `openspec/specs/std_go_tokenbucket_allow/spec.md` ("the library never reads HTTP"). No hay escritura de cabeceras en ninguno de los dos paquetes: `not found`.
- `tokenbucket` `Memory.Allow(ctx, key) (bool, time.Duration, error)` — `tokenbucket/memory.go:65`. Calcula `tokens` (en `entry.tokens`) y `waitMicro` vía `consumeOne` (`tokenbucket/clock.go:56`); devuelve solo `allowed` y `wait`.
- `tokenbucket` `Redis.Allow(ctx, key) (bool, time.Duration, error)` — `tokenbucket/redis.go:52`. El Lua devuelve `{true, wait_duration, tokens}` (`tokenbucket/lua.go`); Go comprueba 3 campos pero solo lee `values[1]` (wait); `values[2]` (tokens) se descarta.
- `tokenbucket` solo conoce `rate` (tokens/s), `burst`, `maxDelay`, `ttl` — `tokenbucket/clock.go:21` (`clockConfig`). No recibe un `period` ni un `average` de configuración.
- `tokenbucket`: con `maxDelay > 0` un consume puede ser `allowed=true` con `wait>0` (admisión diferida); al denegar se reembolsa el token — `tokenbucket/clock.go:56`, `openspec/specs/std_go_tokenbucket_allow/spec.md`.
- `windowcounter` `Take` / `Peek` / `Allow` `(allowed bool, estimated float64, err error)` — `windowcounter/limiter.go:157`, `:173`, `:188`. `Allow` es alias de `Take`.
- `windowcounter` estimado = `current + previous × weight`; solo se devuelve la suma — `windowcounter/limiter.go:193` (`takeExact`), `:214` (`takeBuffered`), `:227` (`peekExact`), `:241` (`peekBuffered`).
- `windowcounter` ventana fija alineada al epoch: `windowStart = floor(now.Unix()/windowSec)×windowSec`, peso calculado en segundos enteros; el fin de la ventana (`windowStart+windowSec`) no se devuelve — `windowcounter/limiter.go:128` (`slidingAt`).
- `windowcounter` Take cuenta el hit aunque deniegue; Peek no incrementa — `windowcounter/limiter.go:157`, `openspec/specs/std_go_windowcounter_sliding-take/spec.md`.
- `windowcounter` modo buffered (`sync_rate > 0`) usa `redisKnown + localDelta` locales y, en caída de Redis, el cupo de esta instancia con `err == nil` — `windowcounter/limiter.go:214`.
- Llamadores Go de estas firmas fuera de los paquetes en este repo: `not found` (solo tests propios, `README.md`, `knowledge/devdocs/`).

## Desired

1. La librería sigue sin escribir HTTP (ninguno de los dos paquetes).
2. `tokenbucket` y `windowcounter` devuelven la información necesaria para que el middleware ponga `Retry-After`.
3. Ambos devuelven también la información para poner `X-RateLimit-Remaining`, `X-RateLimit-Limit`, `X-RateLimit-Period` y `X-RateLimit-Reset`.
4. API (decisión del humano): se cambian las firmas existentes — `tokenbucket` `Memory.Allow` / `Redis.Allow` y `windowcounter` `Take` / `Peek` / `Allow` devuelven `(Status, error)`, con `Status` un struct con esos datos. Cambio rompedor aceptado; sin métodos paralelos.
5. Semántica (decisión del humano): en `windowcounter`, el dato para `X-RateLimit-Reset` es el tiempo hasta el fin de la ventana fija actual (alineada al epoch), no hasta cupo completo.

## Affected

- `tokenbucket/memory.go`, `tokenbucket/redis.go` (firmas y retorno); `tokenbucket/lua.go` / `tokenbucket/clock.go` solo si el dato no basta (ver Unknowns).
- `windowcounter/limiter.go` (firmas de `Take` / `Peek` / `Allow` y rutas `takeExact` / `takeBuffered` / `peekExact` / `peekBuffered`).
- Tests y benchmarks que llaman esas firmas: `tokenbucket/*_test.go`, `windowcounter/*_test.go` (incluidos `*_yaegi_test.go` y `bench_*`).
- Contratos vivos: `openspec/specs/std_go_tokenbucket_allow/spec.md`, `openspec/specs/std_go_windowcounter_sliding-take/spec.md`.
- Documentación de uso: `README.md` (ejemplos de `Peek` / `Take` / `Allow`), `knowledge/devdocs/std_go_tokenbucket.md`, `knowledge/devdocs/std_go_windowcounter.md`.

## Out of scope

- Escribir cabeceras HTTP o el 429 desde la librería (lo prohíbe el punto 1).
- Métodos paralelos o wrappers de compatibilidad con las firmas antiguas (punto 4).
- Semántica Hub `DistributedRateLimit` más allá de los datos pedidos: `denyOnError`, `limit 0 = ilimitado`, `sourceCriterion` (inferido; no pedido).
- `backendbackoff` (`backendbackoff/allow.go`), aunque su doc mencione `Retry-After`: no está nombrado en el pedido.
- Formato textual de las cabeceras (segundos enteros, fecha HTTP, etc.): lo decide el middleware.
- Cambiar el algoritmo de admisión de cualquiera de los dos limitadores.

## Unknowns

- `tokenbucket` Limit / Remaining / Period / Reset: mapeo exacto. Hipótesis del chat, no confirmada: Limit=`burst`, Remaining=`floor(max(tokens,0))`, Period=`burst/rate` (inferencia: la librería no recibe un period), Reset=`(burst−tokens)/rate` (tiempo a bucket lleno), RetryAfter=`wait` al denegar.
- `tokenbucket` con `maxDelay > 0`: qué deben valer RetryAfter y Remaining en una admisión diferida (`allowed=true`, `wait>0`) y en una denegación con reembolso.
- `tokenbucket` Redis: si `values[2]` (`tostring(tokens)` en Lua) basta y es estable en Redis y Dragonfly (formato, NaN/Inf), o si hace falta tocar el script.
- `windowcounter` RetryAfter: fórmula. Hipótesis del chat: forma cerrada desde `current` / `previous` / `weight` / `limit`; hay que separar `current` y `previous` (hoy solo se devuelve la suma). Efecto de que los Take denegados sigan incrementando.
- `windowcounter` Remaining / Limit / Period: hipótesis del chat Remaining=`floor(max(limit−estimated,0))`, Limit=`limit`, Period=`window`. En `Peek` (sin hit) frente a `Take` (con hit).
- `windowcounter` modo buffered: los valores serán locales (`redisKnown + localDelta`) y, en caída de Redis, del cupo de esta instancia; si eso es aceptable como dato de cabecera.
- Precisión de Reset en `windowcounter`: el reloj trabaja en segundos enteros (`now.Unix()`).
- Coste de rendimiento del struct de retorno (asignación, interpretación bajo Yaegi). La memoria del proyecto exige medir en compilado y en Yaegi si hay impacto; los benchmarks de #105 (`bench_test.go`, `bench_yaegi_test.go`) son la línea base.
- Llamadores externos (el middleware) que rompen con el cambio de firma: fuera de este repo, `not found`.

## Tensions

- Pedido vs. decisión previa: el 2026-11-21 (PR de benchmarks) el humano rechazó añadir semántica Hub, incluidas las cabeceras `X-RateLimit-*`, y prefirió no cambiar la API pública. Este pedido lo revierte explícitamente para los datos de cabecera y el cambio de firma; el resto de la semántica Hub sigue fuera.
- Pedido vs. `knowledge/devdocs/std_go_windowcounter.md` (Peek, `_Avoid_: remaining quota`): el doc desaconseja leer Peek como cupo restante; el pedido exige un dato Remaining también en `Peek`.
- Pedido vs. modelo `tokenbucket`: `X-RateLimit-Period` no tiene equivalente directo; la librería solo conoce `rate` y `burst` (`tokenbucket/clock.go:21`). Cualquier Period es derivado.
- Decisión 5 vs. ventana deslizante: al fin de la ventana fija el peso de la anterior no es cero, así que Reset no implica cupo completo. El humano lo decidió así a sabiendas; se registra para que explore no lo trate como error.
- Contratos vivos `std_go_tokenbucket_allow` y `std_go_windowcounter_sliding-take` describen los retornos actuales (`allowed` + wait / estimate); el cambio rompedor los contradice y requiere actualizarlos.
- Hub research (`knowledge/research/ext_traefik_hub_distributed-rate-limit/notes.md`): documenta los nombres de cabecera pero no formatos/unidades ni `Retry-After`, y una de sus páginas omite `X-RateLimit-Period`. No hay referencia externa para la semántica exacta.
