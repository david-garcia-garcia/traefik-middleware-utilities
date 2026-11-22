# Delivery

## Motivation

El `Limiter` de `windowcounter` en modo buffered (`syncRate > 0`) acumula deltas en `l.windows` y un `AfterFunc` (`flushAfterTick`) los envía a Redis. #105 ya agrupa esos EVAL para no sujetar `l.mu` durante Redis. Take y Peek del mismo `Limiter` siguen serializando en ese mutex; `Sleep` y `Close` llaman al mismo flush.

Cada tick recorre todo `l.windows` dos veces con `l.mu` cogido: recoger ventanas con `localDelta > 0` y podar las caducadas. Ambos pasos son O(K) aunque no haya nada sucio ni que caduque. El mapa no tiene tope; K ≈ claves distintas de las últimas 2–3 ventanas (`expireAt = windowStart + 2*window`). El limitador por IP con rotación de direcciones infla K. El timer se rearma al volver del flush, así que el duty cycle del mutex es `scan / (scan + syncRate)`.

Un tick idle (nada que enviar) mide ~11,1 ms compilado y ~51,9 ms bajo Yaegi a 200 000 ventanas vivas, y ~67 ms compilado a 1 000 000. Con `syncRate` 20 ms y 200 000 ventanas, Yaegi deja el mutex ocupado ~72 % del tiempo: cada Take/Peek de ese `Limiter`, de cualquier clave, espera detrás del scan (del orden de 20 ms de más, peor ~52 ms). Lo mismo pagan `Sleep` y `Close`.

Si se deja así, un `Limiter` buffered con muchas claves vivas atasca toda la admisión de esa instancia en cada tick, compilado y como plugin Traefik (Yaegi). No cambia el resultado de admisión; sí el stall, que crece con el número de ventanas vivas.

Priority: P2 — dolor real de operador y usuario final, con radio de impacto limitado

## Implementation

Un índice sucio `dirty map[string]*windowState` guarda las ventanas con `localDelta > 0` (o `flushDelta > 0` mientras un flush está en vuelo). `takeBuffered` las mete solo en la transición 0→1; `applyFlushOutcomeLocked` las saca cuando ambos deltas vuelven a 0. `flushPendingLocked` recorre `l.dirty` en vez de `l.windows`.

Un índice de caducidad `expiry map[int64]map[string]*windowState` agrupa por segundo de `expireAt`. Tres helpers (`putWindowLocked`, `setExpireLocked`, `deleteWindowLocked`) son el único sitio que inserta, mueve o borra en `l.windows` y en los cubos. La poda visita solo cubos vencidos (`expireAt == 0` o `nowUnix >= expireAt`) y borra ventanas idle con la misma condición de siempre.

El coste del tick queda en O(sucias + que caducan ahora + segundos distintos de `expireAt`). Admisión, semántica de flush (`std_go_windowcounter_sync-flush`) y batching no cambian. Un test aleatorio comprueba índices y conteos Redis; otro afirma que el flush idle a 200 000 ventanas queda por debajo de 3× el de 10 000. Hay `BenchmarkFlushIdle` compilado y Yaegi.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** None.
**End users.** None.
