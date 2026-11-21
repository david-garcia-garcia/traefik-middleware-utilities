# Performance

1. [judgement] Trabajo movido dentro del mutex — `windowcounter/limiter.go:555` — `flushBatchEvalUnlocked` construye `keys` y `args` (2 `strconv.FormatInt` por clave más dos slices) antes de `l.mu.Unlock()`; el código anterior formateaba los argumentos después de soltar el lock. En cada tick de `sync_rate` el tiempo con `l.mu` tomado crece en ~3 asignaciones por clave sucia, y Take/Peek en modo buffered esperan ese lock (más caro interpretado bajo Yaegi). `batch` es una subslice de `pending`, una copia local, así que se puede leer sin el lock
   Quote:
   ```
   keys := make([]string, len(batch))
   args := make([]string, 0, 2*len(batch))
   for i, job := range batch {
   	keys[i] = job.redisKey
   	args = append(args, strconv.FormatInt(job.delta, 10), strconv.FormatInt(job.expireAt, 10))
   }
   l.mu.Unlock()
   defer l.mu.Lock()
   ```
   Fix: Mover `l.mu.Unlock()` / `defer l.mu.Lock()` antes del bucle que construye `keys` y `args`
   Status: done
   Argument: `flushBatchEvalUnlocked` suelta `l.mu` antes de construir `keys`/`args` (batch es copia local). Commit `197172e8`.
2. [judgement] Recorrido completo bajo el mutex en cada tick — `windowcounter/limiter.go:488` — `flushPendingLocked` sigue recorriendo todo `l.windows` dos veces con `l.mu` tomado (snapshot de sucias en :488 y purga de caducadas en :524) cada `sync_rate` (mínimo 20 ms). Crece con todas las ventanas en búfer (TTL de dos ventanas, sin tope de tamaño), no solo con las sucias; Take/Peek buffered se bloquean durante el recorrido. Es anterior a este diff (líneas de contexto sin cambios), pero el README nuevo (`README.md:122`) dice "Flush cost grows with dirty keys", que se queda corto
   Note:
   ```
   No medido: BenchmarkFlush usa 1000 ventanas, todas sucias, así que no separa el coste del recorrido del de los EVAL.
   ```
   Fix: Que `README.md:122` diga que cada flush recorre todas las ventanas en búfer, y registrar como deuda un índice de ventanas sucias/por expiración
   Status: skipped
   Argument: judgement: el doble recorrido de `l.windows` es anterior a este diff; acotarlo cambia la estructura del buffer y queda fuera del ticket.
3. [judgement] Drenaje sin acotar por llamada — `tokenbucket/memory.go:107` — `makeRoomLocked` retira de golpe todas las cabezas caducadas: tras un periodo sin fuentes nuevas, un solo `Allow` puede quitar hasta 65535 entradas (unlink + `delete`) con `m.mu` tomado, bloqueando a todos los demás `Allow`. Está acotado por `maxMemorySources` y es O(1) amortizado (cada entrada sale una vez), mejor que el recorrido completo por fuente nueva de antes; pero `README.md:122` dice "O(1)" sin matizar, y el peor caso no se mide (`new-key-full-table` congela el reloj y nunca caduca nada; `TestYaegi_MemoryCapAndExpiry` ejecuta justo este drenaje interpretado, sin cronometrarlo)
   Quote:
   ```
   for m.head != nil && !now.Before(m.head.expireAt) {
   	m.removeLocked(m.head)
   }
   ```
   Note:
   ```
   Acotar el drenaje (p. ej. k cabezas caducadas por fuente nueva) mantiene el tope y "caducadas antes que vivas", porque la cabeza es siempre la que antes caduca; pero choca con el escenario del delta std_go_tokenbucket_allow "Full store drops expired sources before live ones", que exige que todas las caducadas hayan salido tras ese Allow.
   ```
   Fix: Escribir "O(1) amortizado; una fuente nueva retira de una vez todas las fuentes caducadas" en `README.md:122`, o acotar el drenaje por llamada y ajustar ese escenario
   Status: skipped
   Argument: judgement: el drenaje está acotado por el tope y es O(1) amortizado; acotarlo por llamada contradice el escenario del spec que exige quitar todas las caducadas antes de guardar la nueva.
4. [judgement] El benchmark Yaegi no pasa por el camino cambiado — `tokenbucket/bench_yaegi_test.go:323` — `BenchmarkYaegiMemoryAllow/hot-key` siempre toca la cola, así que el atajo `entry != m.tail` se salta `unlinkLocked` + `pushBackLocked` y la fila Yaegi del README (8,567 ns/op) no incluye el movimiento en la lista que el diff añade a cada `Allow` sobre una fuente guardada que no es la última tocada (cualquier tráfico con más de una fuente). El checkpoint B (`ae7f4a9a`, sin el atajo de cola) midió ese movimiento interpretado: mediana de hot-key Yaegi 12,332 ns/op frente a 8,567 en `a51d20a3` y 8,181 antes (`d940bf43`), es decir ~3.8 µs (~45 %) más por `Allow` interpretado. Compilado el coste es despreciable (rotating-10k 36.75 → 38.01 ns/op)
   Quote:
   ```
   bench-after-ae7f4a9a.txt  BenchmarkYaegiMemoryAllow/hot-key  12243 12292 12332 13008 13070 ns/op
   bench-after.txt           BenchmarkYaegiMemoryAllow/hot-key   8450  8497  8567  8739  9345 ns/op
   bench-before.txt          BenchmarkYaegiMemoryAllow/hot-key   7541  7739  8181  8393  9313 ns/op
   ```
   Fix: Añadir una sub-benchmark Yaegi con claves rotativas (`BenchmarkYaegiMemoryAllow/rotating`) y publicar su fila, o decir en `README.md:157` que la fila hot-key no ejecuta el movimiento en la lista
   Status: done
   Argument: Taken: implement (re-entrada) añadió `BenchmarkYaegiMemoryAllow/rotating-10k` (`2c504a9b`) y en `3623aafd` hace el relink en una sola llamada; el README publica 7,549 ns/op frente a 6,733 antes (par consecutivo, dentro del ±11 % que el propio README declara).
5. [judgement] allocs/op atribuidos al limitador incluyen el arnés — `README.md:155` — la columna "Compiled allocs/op" se lee como coste del limitador, pero `b.ReportAllocs` cuenta todas las asignaciones del proceso: en las filas con fake se incluyen las del `testFakeRedis` en proceso (parseo de RESP, `flushBatchReplyLocked` con `fmt.Fprintf` por clave), y en `new-key-full-table` la clave `"new:"+strconv.Itoa(i)` construida dentro del bucle cronometrado. Las cifras (74 allocs/op en `NewRedis`, 15,941 en `BenchmarkFlush`) no miden solo el cliente; la comparación antes/después sigue siendo válida porque el arnés es el mismo
   Fix: Añadir en "How to read the numbers" que allocs/op incluye el fake en proceso y la construcción de claves del bucle, o precomputar las claves de `new-key-full-table` antes de `b.ResetTimer()`
   Status: skipped
   Argument: judgement: allocs/op incluye el arnés en las dos columnas por igual; la comparación antes/después sigue siendo válida.
