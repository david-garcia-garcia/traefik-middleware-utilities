# Delivery

## Motivation

El limitador Redis del token bucket lee el hash con `HGETALL` y asigna `last` al segundo elemento y `tokens` al cuarto. Redis y Valkey no garantizan ese orden: con codificación hashtable depende de la semilla de hash del proceso. La existencia del defecto se toma de https://github.com/traefik/traefik/issues/14297 y del arreglo upstream https://github.com/traefik/traefik/pull/14300.

Si `tokens` llega primero, `last` recibe el número de tokens y `tokens` recibe el reloj en microsegundos. El tiempo transcurrido se vuelve enorme, el bucket se rellena hasta `burst` en cada llamada y se queda en `burst - 1`. Con burst 5 se admitieron 10 de 10 peticiones. El limitador deja de limitar y no lo registra.

Dejarlo así abre el límite en cualquier proceso cuya semilla ponga `tokens` primero, y también cuando el hash se insertó en ese orden.

Priority: P1 — un proceso Redis con `tokens` antes que `last` admite todas las peticiones

## Implementation

El script pide `last` y `tokens` con `HMGET` por nombre. El bucket ya existe solo si los dos valores son truthy; si falta uno, siembra `tokens = burst` y `last` al reloj de la llamada, como hasta ahora. El resto del script (relleno, tope de espera, `HSET`, `EXPIRE`) no cambia. En Redis y en Dragonfly un campo ausente llega como booleano `false`, así que la siembra sigue corriendo.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El script Redis lee `last` y `tokens` por nombre. Un bucket guardado cuenta como presente solo cuando los dos campos son truthy.
**End users.** Un bucket ya lleno vuelve a denegar al pasar de `burst`, también cuando el hash guarda `tokens` antes que `last`.
