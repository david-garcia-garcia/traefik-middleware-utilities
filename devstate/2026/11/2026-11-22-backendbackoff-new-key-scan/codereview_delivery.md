# Delivery

## Motivation

Cada `Allow` y `Report` del `Gate` serializa en un solo `g.mu` durante toda la llamada. El coste que crece con el mapa no está en una clave ya residente: está en admitir una clave que no está en `g.keys`.

Cuando `loadEntry` ve una clave desconocida, `dropExpired` recorre todo el mapa bajo ese mutex. En el tope de 65.536, `dropOne` lo recorre otra vez y borra un slot arbitrario. El coste por clave nueva crece con `len(g.keys)`: en compilado, de 1 µs (mapa vacío) a 1,8 ms (65.536); bajo Yaegi, de 36 µs a 5,1 ms (16.384). En Yaegi, unas 270 claves nuevas/s saturan el mutex (mapa a 16k, TTL 60 s); en compilado, cerca del tope, unas 500/s.

Todo `Allow` y `Report` de todos los backends de ese `Gate` se encola detrás del barrido. Solo pega cuando la clave tiene alta cardinalidad (cliente, IP, ruta dinámica); decenas o cientos de claves por backend no se notan. Es el mismo patrón que #105 ya sustituyó en `tokenbucket.Memory`.

Priority: P2 — dolor real de operador, con workaround o radio de impacto limitado

## Implementation

`memEntry` entra en una lista doblemente enlazada de expiración con punteros concretos (`key`, `prev`, `next`); `Gate` guarda `head` y `tail` (sin `container/list`). Cada escritura de `expireAt` — cada `Allow`, también denies, y cada `Report` que aterriza — mueve la entrada a la cola con `moveToBackLocked` (sin-op si ya es cola; reenlace in situ para no pagar una llamada extra bajo Yaegi). Una clave nueva llama `makeRoomLocked`: saca cabezas caducadas y, si sigue en el tope, saca la cabeza. `Close` anula `head`/`tail` con el mapa. El comportamiento observable sigue `std_go_backendbackoff_allow` (TTL perezoso, refresh en cada `Allow`, tope 65.536, caducadas primero, luego un slot arbitrario). Tests de lista y tope, más benches de clave nueva con tabla llena en compilado y Yaegi; se cierra la nota de deuda del barrido.

## What this changes
**Operators.** Con claves de alta cardinalidad, admitir una clave nueva ya no satura `g.mu` recorriendo el mapa; no hay clave de despliegue nueva.
**Admin users.** None.
**Developers.** Las firmas de `New`, `Allow`, `Report` y `Close` no cambian; admitir una clave nunca vista ya no recorre `g.keys`, y en el tope se sigue expulsando un slot arbitrario.
**End users.** None.
