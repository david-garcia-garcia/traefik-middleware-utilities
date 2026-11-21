# Delivery

## Motivation

Bajo Traefik, `simpleredis` se ejecuta interpretado con Yaegi v0.16.1, la versión que fija `go.mod`. El código de sesión tenía cuatro sentencias `select` en tres sitios. Dos estaban en el semáforo de turnos en uso, que limita los sockets vivos a `PoolSize`: la toma no bloqueante y la espera con pool lleno de `borrowSocket`, y la devolución no bloqueante de `freeInUseTurn`. La tercera estaba en el backoff entre reintentos de `waitUntil`. Las cuatro mezclan canales propios de cada llamada o de cada cliente: el canal `inUseTurns`, `ctx.Done()` y `timer.C`.

En Yaegi v0.16.1, `_select` (`interp/run.go`) reserva una sola lista `[]reflect.SelectCase` por sentencia y no por ejecución. Cada goroutine que llega a esa sentencia sobrescribe la lista sin lock antes de `reflect.Select`. Así, dos goroutines en el mismo `select`, aunque pertenezcan a llamadas o clientes distintos, pueden esperar en los canales de la otra o recibir de ellos. Compilado no ocurre, y por eso la suite unitaria nunca lo vio. Reproducido bajo Yaegi dentro de `go test`, con el control compilado siempre limpio:

- `borrowSocket` devuelve `(nil, nil)` entre 2 y 15 veces por ejecución. Por `Get`, ese `nil` llega a `runOnConn` y después a `release` → `close`, y produce un panic (`reflect: call of reflect.Value.Field on zero Value`, marco Yaegi `resp.go:19`) que tumbó el proceso de test en 0,2 s.
- Dos clientes con 32 turnos cada uno se intercambian turnos. Uno termina con 30 o 31 turnos libres y el otro cuenta over-frees falsos, con su log `simpleredis_over_free`. El turno perdido no vuelve.
- Un backoff de 1 ms termina en `DeadlineExceeded` porque otra goroutine le robó el tick. Ocurrió entre 6 y 28 veces por ejecución.

Si no se corrige, un cliente en Traefik puede, con concurrencia, entrar en pánico en un `Get`, perder capacidad de pool de forma permanente, y registrar `pool_wait` y `simpleredis_over_free` que no corresponden a una sobrecarga real. Los reintentos fallan sin que Redis tenga ningún problema. Además, nada impedía reintroducir un `select` en el código de sesión, y ningún test probaba la concurrencia interpretada de esos sitios.

Priority: P1 — la Motivation describe código de producción bajo Traefik (Yaegi v0.16.1) que hoy puede entrar en pánico y perder turnos del pool de forma permanente.

## Implementation

El canal `inUseTurns` se sustituye por un semáforo con mutex. Sus partes son `inUseMu`, `freeTurns`, `turnCap` (fijado en `New`, y `0` significa que el cliente no salió de `New`) y una cola FIFO `turnWaiters` de `*turnWaiter`. Cada `turnWaiter` lleva un canal `ready` y un campo `granted`. La toma sin contención y la devolución son inline, con `Lock` y `Unlock` explícitos, sin método nuevo, sin `defer` y sin `sync.Once`, para no pagar alrededor de 1 µs por cada llamada interpretada. Un mismo `Lock` en `borrowSocket` toma un turno libre o encola al que pide. `freeInUseTurn` entrega el turno en el sitio al waiter más antiguo (`granted = true`, `close(ready)`). Si no hay nadie esperando, incrementa `freeTurns` hasta `turnCap`. Si ya está lleno, marca un over-free, que se cuenta y se registra fuera del lock. Se cumple el invariante `freeTurns > 0 ⇒ cola vacía`, así que una toma posterior nunca adelanta a un waiter.

En la espera con pool lleno, el waiter hace una recepción simple `<-waiter.ready`. Un único cierre `abandon` lo retira de la cola bajo `inUseMu` y cierra `ready`. Ese cierre se arma dos veces, con `time.AfterFunc(PoolTimeout)` y con `context.AfterFunc(ctx)`. Como la concesión y el abandono van bajo el mismo lock, el primero que llega cierra `ready` y el otro ya no encuentra al waiter, de modo que no se pierde ni se duplica ningún turno. La clasificación de la salida no cambia: con el turno concedido, la llamada sigue; si el ctx ha vencido, se devuelve `ctx.Err()` sin log; en otro caso, se registra `pool_wait` y se devuelve `errPoolWait`. El diseño proponía `context.WithTimeout` más un `context.AfterFunc`, pero en la medición salió más caro y se entregó la alternativa prevista. `waitUntil` espera en `<-Done()` de un hijo `context.WithTimeout(ctx, delay)`.

Para la regresión hay tres piezas. La primera es una guarda AST determinista, `TestSessionSourceHasNoSelect`, que falla con `archivo:línea` si aparece un `*ast.SelectStmt` en el código no test del paquete. Incluye un caso en memoria para que no pueda pasar en vacío. La segunda son cuatro pares `TestReproSelect_*_Compiled` / `_Yaegi` (`WaitUntil`, `BorrowWait`, `TwoPools`, `SaturatedGet`), que ejecutan el código real bajo Yaegi. Los `_Yaegi` se saltan con `-race`. La tercera son tests compilados del semáforo en `pool_test.go`: entrega FIFO, carrera entre concesión y abandono, cancelación antes de `PoolTimeout`, y `PoolTimeout` que devuelve `ErrPoolWait`. El alcance se limita a `simpleredis`. El `select` de `reclaim` `waitCtx` y la anomalía de Yaegi con `switch` sin etiqueta y `case` múltiple quedan registrados como deuda.

## What this changes
**Operators.** Bajo Traefik, `pool_wait` y `simpleredis_over_free` solo aparecen ya por saturación o devoluciones reales, no por cruces entre goroutines. No hay claves ni configuración nuevas.
**Admin users.** None.
**Developers.** El código de sesión de `simpleredis` no puede contener `select` (lo vigila `TestSessionSourceHasNoSelect`), un turno devuelto pasa al waiter más antiguo en orden FIFO, y los tests de caja blanca leen el semáforo con `inUseTurnsForTest` y `holdTurnForTest` en lugar de `inUseTurns`. La API exportada, los sentinels y los textos de error no cambian.
**End users.** None.
