# Test coverage

1. [hard] Ruta crítica sin test — `windowcounter/limiter.go:625` — un Take durante el EVAL deja `localDelta > 0`; ningún test falla si `delete(l.dirty)` corre igual
   Quote:
   ```
   windowcounter/limiter.go:625-627
   if state.localDelta == 0 && state.flushDelta == 0 {
       delete(l.dirty, job.redisKey)
   }

   windowcounter/zzz_limiter_flush_batch_test.go:153-156
   TestFlush_TakeDuringBatchKeepsHit asserts redisKnown==3, localDelta==2, flushDelta==0;
   no dirty membership; no second flush / Redis count of 5.

   windowcounter/zzz_flush_dirty_index_test.go:106-154
   TestFlushDirtyIndex_RandomizedIndexesAndCounts is sequential; no in-flight Take.
   ```
   Fix: Tras Takes en vuelo, afirmar que la clave sigue en `dirty` y que el siguiente `flushPending` escribe esos hits en Redis
   Status: done
   Argument: `TestFlush_TakeDuringBatchKeepsHit` afirma `dirty` y Redis 5 tras el segundo flush.
2. [hard] Caso límite sin test — `windowcounter/limiter.go:571` — el tick idle no reafirma que las ventanas no vencidas sigan vivas tras `flushPending`
   Quote:
   ```
   windowcounter/limiter.go:570-577
   for expireAt, bucket := range l.expiry {
       if expireAt != 0 && nowUnix < expireAt {
           continue
       }
       for redisKey, state := range bucket {
           if state.localDelta == 0 && state.flushDelta == 0 {
               l.deleteWindowLocked(redisKey, state)
           }
       }
   }

   windowcounter/zzz_flush_dirty_index_test.go:178-188
   nsPerIdleFlushFlushDirtyIndex checks len(windows)==liveWindows only before the warmup flush.
   ```
   Fix: Tras el flush idle, afirmar que `len(windows)` sigue siendo 10k/200k
   Status: done
   Argument: `assertIdleWindowsFlushDirtyIndex` tras warmup y muestras; el sample loop espera 50ms para no leer 0s.
3. [judgement] Solo camino feliz — `windowcounter/limiter.go:575` — no hay aserción de que una ventana idle vencida salga de `windows`
   Quote:
   ```
   windowcounter/limiter.go:574-577
   for redisKey, state := range bucket {
       if state.localDelta == 0 && state.flushDelta == 0 {
           l.deleteWindowLocked(redisKey, state)
       }
   }

   (none) — checkIndexesLockedFlushDirtyIndex does not require expired idle keys to be gone
   ```
   Fix: Avanzar el reloj past `expireAt`, hacer flush y afirmar que la ventana idle ya no está en `windows`
   Status: skipped
   Argument: judgement; la poda idle vencida ya existía; este cambio solo la indexa.
