# Spec

1. [wrong] Caducadas primero con reloj que retrocede — `tokenbucket/memory.go:106` — el requisito ADDED de `std_go_tokenbucket_allow` es incondicional: una fuente viva solo sale cuando la tabla sigue en el tope y no queda ninguna caducada "by the limiter clock". `makeRoomLocked` solo saca caducadas mientras la cabeza de la lista esté caducada y después desaloja la cabeza viva. Con un reloj que retrocede (`SetNowForTest`, método público), puede quedar una caducada detrás de una cabeza viva. En ese caso, con 65536 fuentes, se desaloja una viva y la caducada sigue guardada. Antes, `dropExpired` recorría todo el mapa y la promesa se cumplía. Design D3 acepta el desorden ("the head may not be the first to expire... Accepted"), pero ni el spec ni su escenario recogen esa condición. El devdoc `std_go_tokenbucket.md` dice "without changing any decision", y eso no es cierto en el tope. Con `time.Now` (lectura monotónica) el orden se mantiene, así que en producción se cumple.
   Quote:
   ```text
   openspec/changes/ratelimit-benchmarks-key-scaling/specs/std_go_tokenbucket_allow/spec.md:4
   Before storing a new source (missing or TTL-expired key), it SHALL remove sources whose `ttl` has elapsed, and SHALL remove a live source only when the store is still at the cap with no elapsed source left.
   ```
   Fix: Acotar el requisito en el spec delta a un reloj del limitador que no retrocede (como acepta D3) y corregir la frase del devdoc para decir que, con reloj de test hacia atrás y la tabla en el tope, puede salir una fuente viva aunque quede una caducada
   Status: done
   Argument: Requisito acotado en `specs/std_go_tokenbucket_allow/spec.md` (el orden vale mientras el reloj del limitador no retrocede; un reloj de test hacia atrás MAY dejar una caducada al desalojar una viva en el tope) y frase corregida en `knowledge/devdocs/std_go_tokenbucket.md`. Sin cambio de código. Commit `197172e8`.
