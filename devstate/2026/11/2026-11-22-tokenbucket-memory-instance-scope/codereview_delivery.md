# Delivery

## Motivation

Cuando varios routers de Traefik apuntan al mismo middleware RateLimit, el limitador en memoria y el de Redis no comparten el mismo ámbito. Traefik construye un limitador in-memory por router, de modo que N routers dan N presupuestos por origen; Redis clavea por middleware (`rate:<middleware>:<source>`) y comparte uno. Esta librería ya replica ese corte: cada `*Memory` es su propio mapa de cubos; `Redis.Allow` evalúa la `key` opaca del llamante en el servidor. El defecto no se redescubrió aquí: viene de Traefik [#13706](https://github.com/traefik/traefik/issues/13706), con el PR abierto de opción `scope` [#13823](https://github.com/traefik/traefik/pull/13823) y el PR cerrado [#13707](https://github.com/traefik/traefik/pull/13707).

El hueco era de contrato, no de código. El GoDoc de `Memory` / `NewMemory` no decía que cada valor sea un espacio de claves ni cómo compartir presupuesto entre routers o recargas. El de `Redis` / `NewRedis` no decía que el estado vive en el servidor, que cualquier limitador sobre la misma clave lo comparte, ni que el prefijo define el ámbito. El catálogo vivo `std_go_tokenbucket_allow` exigía que el prefijo de `key` sea del llamante, pero no nombraba Memory-por-instancia frente a Redis-por-clave. El packet de uso ya afirmaba el share de Redis y omitía el hecho simétrico: dos `*Memory` sobre la misma clave admiten `2*burst`. Tampoco había prueba que afirmara ese contrato: el repro de referencia esperaba un burst compartido entre dos `NewMemory` y medía 6 admisiones frente a burst 3.

Si se deja sin nombrar, un llamante trata el desacuerdo de Traefik como bug de esta librería, y un consumidor que llama `NewMemory` una vez por router obtiene N presupuestos en memoria y uno en Redis para el mismo límite configurado. Cambiar de almacén entonces aprieta el límite sin tocar la config. Traefik midió límites decenas o cientos de veces más holgados que lo configurado cuando muchos routers comparten un middleware. El arreglo a nivel middleware (un `*Memory` por nombre, al estilo #13823) no cabe en este paquete.

Priority: P3 — spec, documentación, tests o claridad interna, sin daño actual para usuarios u operadores.

## Implementation

Sin cambio de `Allow` ni de constructores. El ámbito existente pasa a ser promesa escrita y comprobada en `tokenbucket`.

GoDoc de `Memory` / `NewMemory`: cada valor es su propio espacio de claves; `Allow` en uno no consume tokens de otro; para compartir presupuesto entre routers o recargas hay que reutilizar un `*Memory` (ejemplo: tabla `reclaim` claveada por nombre de middleware, sin importar `reclaim` desde este paquete). GoDoc de `Redis` / `NewRedis`: el estado vive en el servidor; cualquier `*Redis` sobre la misma clave comparte el presupuesto; el llamante prefija la clave para elegir el ámbito.

Los tests autocontenidos, con helpers sufijo `13706` y la API dest `(Status, error)`, afirman ese contrato: dos llamantes que comparten un `*Memory` admiten `burst` en total; dos `*Memory` distintos admiten `2*burst` (comentario que cita `traefik/traefik#13706`); en vivo, dos `*Redis` sobre una clave admiten `burst` en total, con el mismo gate que `TestLive_Limiter`. El packet de uso gana el gotcha simétrico (dos `*Memory` = `2*burst`) junto al share de Redis, y el catálogo `std_go_tokenbucket_allow` registra las mismas dos promesas. El arreglo de middleware queda como nota de deuda: el consumidor debe compartir un `*Memory` si quiere que memoria y Redis coincidan.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** El contrato de `Allow` ahora nombra el ámbito: cada `*Memory` es su propio espacio de claves (dos valores sobre la misma `key` admiten `2*burst`; hay que compartir un valor para un presupuesto, p. ej. `reclaim` por nombre de middleware); cualquier `*Redis` sobre la misma `key` comparte el estado del servidor (admiten `burst` en total); el prefijo sigue siendo del llamante. El cableado a nivel middleware no está en esta librería.
**End users.** None.
