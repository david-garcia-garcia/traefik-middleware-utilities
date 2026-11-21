# Test coverage

1. [hard] Edge case untested — `tokenbucket/memory.go:86` — el movimiento al final de la lista de una fuente ya almacenada que no es la cola no lo prueba ningún test; sin él, una fuente caliente queda en la cabeza, `makeRoomLocked` se detiene en ella y con la tabla llena desaloja esa fuente viva mientras quedan caducadas detrás (viola "caducadas primero" y la fuente vuelve como bucket lleno). Mutación `else if false && entry != m.tail` en copia: `go test -short ./tokenbucket/` sigue en `ok`. Los tests del tope rellenan con `storeMemEntryForTest` o con claves nuevas (`allowAll`) y nunca reusan una clave que no sea la cola.
   Quote:
   ```
   } else if entry != m.tail {
       // Its expireAt is about to become the latest, so it moves to the tail. The tail itself stays put (hot key).
       m.unlinkLocked(entry)
       m.pushBackLocked(entry)
   }
   test: (none) — TestMemory_FullStoreDropsExpiredBeforeLive, TestMemory_FullStoreOfLiveStaysAtCap, TestYaegi_MemoryCapAndExpiry pasan con la rama anulada
   ```
   Fix: Añadir a `memory_test.go` un test que admita "hot" primero, llene la tabla, vuelva a llamar Allow sobre "hot" (gasta su token), deje caducar el resto y, tras Allow de una clave nueva, afirme que "hot" sigue almacenada y deniega
   Status: done
   Argument: `TestMemory_ReusedSourceMovesBehindExpiring` en `tokenbucket/memory_test.go`; falla si se anula `moveToBackLocked` (mutación comprobada). Commit `197172e8`.
2. [hard] Critical path untested — `windowcounter/limiter.go:33` — el brazo por clave del script Lua (`redis.pcall` + marcador `err:`) que evita el doble conteo no lo ejecuta ningún test: el falso reimplementa el script en Go (`flushBatchReplyLocked`, empareja por el texto de `flushBatchScript`) y el test live solo cubre el caso sano. Mutación `redis.pcall(` → `redis.call(` en copia: `go test -short ./windowcounter/` sigue en `ok`.
   Quote:
   ```
   local value = redis.pcall("incrby", KEYS[i], ARGV[2 * i - 1])
   if type(value) == "table" then
     value = "err:" .. tostring(value.err)
   test: TestFlush_BatchOneKeyFailsOthersApply usa fake.failNextFlushIncrBy (Go); TestLive_FlushBatchSeveralKeys solo afirma count == 2 sin fallo
   ```
   Fix: En `limiter_flush_batch_e2e_test.go`, antes del flush poner con SET un valor no entero en la clave de ventana de una de las claves del lote y afirmar que esa clave conserva `localDelta`, las demás quedan aplicadas una vez y el error queda guardado, en Redis y Dragonfly
   Status: done
   Argument: `TestLive_FlushBatchFailedKeyKeepsDelta` en `windowcounter/limiter_flush_batch_e2e_test.go` (SET no entero en una clave del lote); verde en Redis 7 y Dragonfly v1.40.2 locales; con `redis.call` falla en ambos (Dragonfly cuenta dos veces). Commits `197172e8` y `c6b29714` (helper `runForEachLiveEngine` sin parámetros constantes, `unparam`).
3. [hard] Edge case untested — `windowcounter/limiter.go:500` — la división en lotes de `flushBatchKeys` (y el techo de 1024 claves por EVAL del spec) no la prueba ningún test: `TestFlush_ManyKeysFewerEvalsThanKeys` usa 1000 claves, que caben en un solo EVAL bajo 1024, y el test live solo afirma los conteos. Mutación `flushBatchKeys = 1 << 30` en copia: `go test -short ./windowcounter/` sigue en `ok`.
   Quote:
   ```
   for batchStart := 0; batchStart < len(pending); batchStart += flushBatchKeys {
   test: if len(counts) >= dirtyKeys { ... }  if keysInEval > 1024 { ... }  (1000 claves)
   ```
   Fix: En `TestFlush_ManyKeysFewerEvalsThanKeys` afirmar que cada EVAL lleva como mucho `flushBatchKeys` claves y que hay ⌈1000/`flushBatchKeys`⌉ EVALs
   Status: done
   Argument: `TestFlush_ManyKeysFewerEvalsThanKeys` afirma ⌈1000/`flushBatchKeys`⌉ EVALs y como mucho `flushBatchKeys` claves por EVAL. Commit `197172e8`.
4. [judgement] Happy path only — `windowcounter/limiter.go:500` — bajo Yaegi el flush por lotes solo se ejecuta vía `TestYaegi_BufferedShareSleepDoesNotHang` (`BufferedShare`: una clave por EVAL, falso sano); el lote con varias claves, la ranura `err:` (`parseFlushSlot` con `strings.HasPrefix`) y el fallo por número de ranuras no se interpretan, aunque "seguro bajo Yaegi" es restricción del requisito (Desired 4) y `tokenbucket` sí tiene `TestYaegi_MemoryCapAndExpiry`.
   Quote:
   ```
   test: takeprobe.BufferedShare — a.Take/b.Take sobre la misma clave, a.Sleep(), b.Sleep(), espera "last-write-wins" ausente
   ```
   Fix: Añadir una sonda Yaegi contra el falso que tome dos claves, inyecte `failNextFlushIncrBy` en una, llame `Sleep` y devuelva "ok" solo si la otra quedó aplicada y la fallida se reenvía una vez en el siguiente flush
   Status: skipped
   Argument: judgement: el flush por lotes se interpreta bajo Yaegi vía `BufferedShare` y el código nuevo no usa construcciones de riesgo Yaegi (`go`+`select`, `any`); una sonda Yaegi de fallo por clave se deja como mejora.
