# Delivery

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
