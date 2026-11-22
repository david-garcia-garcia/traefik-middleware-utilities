# Delivery

## Motivation

Traefik reconstruye un cliente Redis en cada recarga de middleware y no lo cierra. El limitador Redis de este árbol usa simpleredis, no go-redis: New no arranca trabajo de fondo, Close drena los sockets ociosos, y un cliente silencioso que sigue alcanzable puede conservar ociosos hasta Close. La fuga upstream es https://github.com/traefik/traefik/issues/13618 (PR abierto https://github.com/traefik/traefik/pull/13619): desde go-redis v9.12 cada UniversalClient huérfano fija un CircuitBreakerManager.cleanupLoop, así que goroutines y memoria crecen con cada recarga.

Esa fuga es de Traefik. Aquí no ocurre: simpleredis no arranca ese bucle, y los clientes abandonados sueltan los sockets cuando dejan de ser alcanzables. Falta la prueba de ese contrato con el mismo patrón de recarga: cincuenta veces un cliente y un limitador Redis nuevos, un Allow, y se descartan sin Close. El camino de ciclos de Close del catálogo vivo solo cubre New, uso y Close, no el descarte sin Close.

Sin esa prueba, un New posterior que arranque una goroutine de fondo, o un pool que conserve sockets proxied después de que el cliente pase a basura, enviaría la misma clase de fuga por recarga que Traefik, sin candado live en Redis ni Dragonfly.

Priority: P3 — tests and catalog proof with no current user or operator harm

## Implementation

Una regresión live en el paquete tokenbucket simula esas cincuenta recargas a través de un proxy TCP de loopback que solo cuenta sockets abiertos (accept más copia bidireccional; sin retención de escrituras). Cada recarga construye simpleredis.New en la dirección del proxy, NewRedis, un Allow con la firma existente (Status, error), y suelta ambos sin Close. Tras asentar el GC, el recuento de sockets abiertos del proxy debe ser 0 y las goroutines filtradas (después menos antes menos tres por socket abierto restante, excluyendo las del propio proxy) fallan solo cuando superan 2. El test reutiliza el gate live existente para que Redis y Dragonfly lo ejecuten, y cierra el cliente de espera antes del bucle de recarga para que ese socket no entre en el conjunto descartado. Un requisito aditivo en std_go_simpleredis_tcp-session registra que los clientes abandonados sin Close no dejan sockets tras el GC y que New sigue sin arrancar una goroutine de fondo, sin cambiar idle-hasta-Close para un cliente que sigue alcanzable.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Un cliente SimpleRedis inalcanzable descartado sin Close no debe dejar sockets TCP tras el GC, y New sigue sin arrancar una goroutine de fondo.
**End users.** None.
