# Delivery

## Motivation

Quien ya conoce el limitador in-memory sabe que hay un bucket por fuente opaca, un tope de 65536 y una lista de caducidad; el camino Redis termina cada EVAL con `EXPIRE` en segundos enteros. El síntoma que origina este trabajo está en Traefik: https://github.com/traefik/traefik/issues/13704. Allí `ttlmap` solo libera una clave caducada si alguien la vuelve a consultar, o cuando el mapa llega a 65536 entradas y entonces libera solo una; la memoria crece con el número de fuentes distintas vistas, no con las activas.

Ese fallo no se da en este árbol: Memory ya no retiene buckets caducados cuando entra una fuente nueva, y Redis/Dragonfly ya ponen caducidad en cada escritura. Lo que faltaba era la prueba. Los tests del tope 65536 cubren el mapa lleno, no diez fuentes ociosas más allá de `ttl` más una fuente nueva bajo el tope. El e2e vivo admite y deniega, pero no lee el `PTTL` de la clave del bucket.

Sin ese ancla, un cambio posterior en la purga al alta o en el `EXPIRE` del script podría volver a retener fuentes ociosas —Memory creciendo con cada clave distinta, claves Redis sin TTL— y ni el tope 65536 ni el admit/deny lo detectarían.

Priority: P3 — tests y claridad de spec, sin daño actual a usuarios u operadores

## Implementation

Se ancla el comportamiento que el árbol ya tiene, sin tocar `Memory.Allow`, `makeRoomLocked` ni el `EXPIRE` del EVAL.

En Memory, un reloj de test congelado admite diez claves sintéticas, salta `ttl+9s`, admite `"fresh"` y comprueba bajo el mutex que el mapa guarda exactamente un bucket, el de la clave nueva.

En Redis y Dragonfly reales, el mismo gating que el e2e ya existente (`TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`) admite tres veces con `ttl` 7s y lee `PTTL` por EVAL sobre `KEYS[1]`; el valor debe quedar en `(0, ttl]`. Los helpers van en el propio fichero e2e con sufijo `13704`.

El catálogo vivo recibe un escenario bajo el requisito de tope y caducadas: diez fuentes ociosas más una nueva dejan una fuente almacenada. No hay SHALL nuevo de `PTTL`; el `EXPIRE` entero ya estaba.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Quien mantenga el almacén Memory o el EVAL Redis debe conservar que diez fuentes ociosas más allá de `ttl` más una nueva dejen exactamente una fuente, y que la clave del bucket tenga `PTTL` en `(0, ttl]` tras `Allow`.
**End users.** None.
