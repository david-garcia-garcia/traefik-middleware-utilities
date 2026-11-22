# Delivery

## Motivation

El middleware de rate limit llama a `Allow` y traduce el error a HTTP. Traefik responde 500 cuando el cliente ya se ha ido porque su `select` no distingue una cancelación de un error del servidor. La existencia de ese defecto se toma de los informes upstream https://github.com/traefik/traefik/issues/14173, https://github.com/traefik/traefik/issues/13879, https://github.com/traefik/traefik/pull/13880 y https://github.com/traefik/traefik/pull/14236, no de un hallazgo local independiente.

En esta librería el 500 no ocurre: `Allow` ya devuelve `context.Canceled` con `Allowed` falso y el paquete no escribe HTTP. Lo que falta es el contrato escrito y la prueba de dos caminos. En `Memory`, un `ctx` ya terminado no cobra, de modo que el siguiente `Allow` sigue teniendo el burst completo; eso no estaba dicho ni cubierto más allá de un `Status` a cero. En `Redis`, un retorno `context.Canceled` o `DeadlineExceeded` no prueba que el script no haya corrido: si la petición se cancela cuando el `EVALSHA` ya está en el cable, `Allow` vuelve pronto con `Canceled` pero el servidor puede ejecutar el script y cobrar un token.

Si se deja así, quien mapee `Canceled` a 499 puede perder un token de un cliente que ya se fue, y un cambio futuro en `Memory` podría empezar a cobrar una petición cancelada y recortar el burst del siguiente `Allow`. El 500 de Traefik sigue siendo el mapeo del middleware, no de este paquete.

Priority: P3 — documentación y tests, sin daño actual a usuarios u operadores

## Implementation

Sin cambio de comportamiento de `Allow`. Se amplía el GoDoc de `Memory.Allow` (un `ctx` ya terminado devuelve su error y no cobra) y de `Redis.Allow` (`Canceled` / `DeadlineExceeded` no prueba que el script no haya corrido; el token puede haberse cobrado). Ambos comentarios sitúan el mapeo HTTP (499 si `Canceled`, no esperar si `wait==0`) en el middleware y citan traefik/traefik#14173.

`TestTraefik14173_MemoryCanceledContext` afirma `Canceled`, `Allowed` falso, `Status` cero y burst completo después. `TestTraefik14173_RedisCanceledInFlight` cancela `Allow` mientras un proxy TCP retiene el `EVALSHA` (hold 1500 ms, cancel a los 200 ms), afirma retorno en menos de la mitad del hold y registra si se cobró el token, sin un bucle Lua ocupado con `TIME`. Helpers propios con sufijo `14173`; misma puerta live que `TestLive_Limiter`.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El GoDoc de `Memory.Allow` y `Redis.Allow` fija el contrato de cancelación: `Memory` no cobra si el `ctx` ya terminó; en `Redis`, `Canceled` o `DeadlineExceeded` no prueba que el script no haya corrido. El mapeo HTTP 499 pertenece al middleware (traefik/traefik#14173).
**End users.** None.
