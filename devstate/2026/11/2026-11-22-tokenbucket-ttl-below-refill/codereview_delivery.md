# Delivery

## Motivation

Un bucket inactivo que caduca se crea otra vez lleno. Con 30 peticiones por minuto y burst 30, el TTL que calcula Traefik es 3 s, pero el bucket tarda 60 s en llenarse. Medido en Memory: se vacía el burst, pasan 5 s en el reloj, y entran otras 30 peticiones en lugar de 2. Redis y Dragonfly hacen lo mismo porque un hash caducado se siembra con tokens iguales a burst.

El defecto se toma de https://github.com/traefik/traefik/issues/13957. No hay pull request upstream.

Si se deja así, un cliente que espera un poco más que ese TTL recupera el burst entero.

Priority: P1 — el límite admite un burst nuevo tras una pausa más corta que el rellenado

## Implementation

Tras validar el ttl del llamador, `NewMemory` y `NewRedis` guardan en el reloj compartido el máximo entre ese ttl y el techo de rellenado: los segundos enteros de burst/rate más maxDelay, sin pasar del mayor `time.Duration` en segundos enteros. Memory y Redis caducan con ese único valor. La firma no cambia y un ttl fraccionario sigue rechazándose.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El argumento `ttl` de `NewMemory` y `NewRedis` es un mínimo: la librería lo sube al tiempo de rellenado y los dos almacenes caducan con ese valor.
**End users.** Tras una pausa corta ya no reciben un burst nuevo.
