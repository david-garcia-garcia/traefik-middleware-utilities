# Delivery

## Motivation

OpenTyped pide el valor de una clave de Table ya tipado como T. El bind ocurre primero, por el mismo camino que OpenWithHooks: incrementa holders y, si el ctx del llamante tiene Done, registra un AfterFunc hacia drop. Recién después hace stored.(T).

En un mismatch devolvía el zero T y el error `reclaim: open %q: want %T, got %T` sin soltar ese holder. El llamante no obtuvo valor y su ctx seguía manteniendo la encarnación despierta. Cada reintento con el mismo ctx sumaba otro holder y otro AfterFunc. Cien OpenTyped[int] fallidos sobre una clave cuyo create guarda un string dejan holders=100 y Close no corre con Grace 0. El escenario live de mismatch y el test de error solo pedían zero y el texto; no medían holders ni Close.

Si se deja así, Sleep y Close no pueden correr mientras viva ese ctx. Un retry o un ctx largo clava el valor y acumula watchers. El disparo realista es dos consumidores que comparten una Table y la misma clave con tipos distintos, o un error de tipo en el llamante.

Priority: P2 — fuga real de holders y Close que no corre, radio limitado al mismatch de OpenTyped

## Implementation

El camino interno de bind ahora devuelve un bindLease (clave, encarnación, stop del AfterFunc, marca dropped). OpenWithHooks descarta el lease y conserva su firma. OpenTyped, si el assert falla, llama releaseLease y sigue devolviendo el mismo error. OpenTyped sigue siendo una call expression; no hay forma genérica nueva.

releaseLease para el AfterFunc cuando hay stop y luego dropOnce. dropOnce, bajo el mutex de la tabla, marca el lease y corre drop a lo sumo una vez, de modo que el release y el watcher (AfterFunc o watch con Done nil) no pueden soltar el mismo holder dos veces. finishBind con el ctx ya hecho sigue dropeando al momento y no entrega lease. Un create que ya publicó no se deshace: solo se va el holder de esta llamada; si no queda otro, el drop normal duerme o cierra a grace 0.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Un OpenTyped con tipo incorrecto ya no deja al llamante como holder: el recuento vuelve atrás, su ctx no mantiene el valor, y si era el último holder Close corre a grace 0 aunque el ctx siga vivo. Firma, texto de error y el OpenTyped con éxito no cambian; Open y OpenWithHooks tampoco.
**End users.** None.
