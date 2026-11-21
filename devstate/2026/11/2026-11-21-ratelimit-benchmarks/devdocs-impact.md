# Devdocs impact
change: ratelimit-benchmarks-key-scaling

Pin: `origin/master...f6b1598e` (40 rutas revisables, 29 excluidas de `devstate/`).

## Units
- Token bucket, almacén en memoria (`NewMemory`: tope 65536, caducadas primero, lista de caducidad) — subsystem — `tokenbucket/memory.go`, `std_go_tokenbucket_allow` → packet `std_go_tokenbucket`
- Window counter, flush buffered por lotes (K = 256, techo 1024, resultado por clave con `redis.pcall`) — subsystem — `windowcounter/limiter.go`, `std_go_windowcounter_sync-flush` → packet `std_go_windowcounter`

## Findings
none.

Comprobado contra el código final:
- `std_go_tokenbucket`: el Gotcha de expulsión (caducadas desde la cabeza, una viva solo en el tope, LRU no especificado, reloj de test hacia atrás) coincide con `makeRoomLocked` / `moveToBackLocked`; `time.Now` lleva lectura monótona, así que producción mantiene caducadas primero. Comando de benchmark, `zz_bench_hooks.go` y `TOKENBUCKET_LIVE_*` coinciden con `bench_yaegi_test.go` / `bench_e2e_test.go`.
- `std_go_windowcounter`: lotes de `flushBatchKeys` = 256, ranura `err:` por clave, fallo de lote completo o recuento de ranuras distinto falla todas las claves del lote, primer error guardado en `lastOutageErr`, `l.mu` liberado por lote. `limiter_flush_batch_e2e_test.go` y `WINDOWCOUNTER_LIVE_*` presentes.
- `std_go_test-suites`: los benchmarks compilan en CI y no corren; `-bench Live` se salta con `-short` (`liveEngineAddrs`).
- `std_go_simpleredis`: ya dice compilado y Yaegi; el spec vivo `std_go_simpleredis_resp-commands` sigue con la premisa "solo compilado" (deuda ya anotada en `issues.md`, no se duplica).
- README: `## Rate limiters compared`, introducción, `## Yaegi` y `## Tests` coinciden con el código (⌈claves/256⌉ EVAL, O(1) en fuente nueva, fake con caché de scripts caliente). Ninguna frase de README ni de `knowledge/devdocs/` dice ya "solo interpretado" ni un EVAL de flush por clave en presente.

Sin unidad (omitido por la regla del skill): ficheros de benchmark y fakes (tests), notas de `knowledge/research/`, `openspec/changes/` (historia del cambio).

Fuera del alcance de devdocs, sin fila: el comentario de `.goreleaser.yml:4` ("library interpreted by Traefik Yaegi, not a compiled plugin") explica por qué no hay binario y sigue siendo cierto; el comentario de test `simpleredis/interpretedcost_test.go:331` ("Production is compiled into Traefik") es la misma premisa que la deuda ya abierta del spec de SimpleRedis.
