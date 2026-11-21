# Scope

1. [judgement] Scope vs Requirements — `README.md:162` — 2026-11-21-ratelimit-benchmarks: el antes/después de las dos optimizaciones del Desired 4 (Desired 1, cuarto punto) solo se mide compilado. `BenchmarkMemoryAllow/new-key-full-table` y `BenchmarkFlush/dirty-keys-1000` no tienen variante Yaegi (filas 166 y 177 con `—` en "Yaegi ns/op", y el "Before" solo da ns/op compilado), y `bench_yaegi_test.go` en `tokenbucket/` y `windowcounter/` solo cubre la clave caliente, que no pasa por el alta de fuente nueva ni por un flush de muchas claves. Sí hay benchmark compilado y Yaegi para cada limitador (`NewMemory`, `NewRedis`, `sync_rate` 0 y > 0), así que el tercer punto del Desired 1 se cumple al pie de la letra; lo que queda abierto es si el antes/después también debía darse bajo Yaegi, como sugiere el ticket.
   Fix: Añadir variantes Yaegi de `new-key-full-table` y de un flush de muchas claves, con su antes/después en la tabla, o dejar escrito en el README que el antes/después de las optimizaciones solo se mide compilado y por qué
   Status: skipped
   Argument: judgement: añadir variantes Yaegi de `new-key-full-table` y `BenchmarkFlush` exige volver a medir antes/después en la máquina de referencia y ampliar la tabla del README; queda como decisión del solicitante.
   Quote:
   ```
   ticket/source.md:5 — "Por eso cualquier medida de rendimiento debe darse en las dos modalidades."
   requirement.md:46 — "Los benchmarks deben mostrar el antes y el después de las dos optimizaciones del punto 4."
   ```
