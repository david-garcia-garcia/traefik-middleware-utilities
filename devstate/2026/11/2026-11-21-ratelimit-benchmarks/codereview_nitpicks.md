# Nitpicks

1. [hard] Linear coding — `tokenbucket/memory.go:86` — `Allow` re-deriva en el llamador una decisión de la lista de expiración (la cola ya está al final y no se mueve) con `else if entry != m.tail` más `unlinkLocked` + `pushBackLocked`; el lector tiene que conocer el invariante de la lista para ver por qué se salta la cola
   Fix: extraer `moveToBackLocked(entry *memEntry)` con la guarda `if entry == m.tail { return }` dentro y después `unlinkLocked` + `pushBackLocked`; en `Allow`, `else { m.moveToBackLocked(entry) }` (mismo comportamiento)
   Status: done
   Argument: `moveToBackLocked` con la guarda de cola dentro; `Allow` llama `else { m.moveToBackLocked(entry) }`. Mismo comportamiento (tests y Yaegi en verde).
   Quote:
   ```go
   if entry == nil {
   	// Drop expired sources (and one live source at the cap) so the map cannot grow without bound.
   	m.makeRoomLocked(now)
   	// Missing or TTL-expired key is a full bucket, then consumeOne.
   	entry = &memEntry{tokens: float64(m.clock.burst), last: nowMicro, key: key}
   	m.buckets[key] = entry
   	m.pushBackLocked(entry)
   } else if entry != m.tail {
   	// Its expireAt is about to become the latest, so it moves to the tail. The tail itself stays put (hot key).
   	m.unlinkLocked(entry)
   	m.pushBackLocked(entry)
   }
   ```
2. [hard] Symmetry and consistency — `tokenbucket/memory.go:83` — la baja de una fuente tiene dueño (`removeLocked`: `unlinkLocked` + `delete(m.buckets, entry.key)`), pero el alta simétrica (fijar `key`, `m.buckets[key] = entry`, `m.pushBackLocked(entry)`) queda inline en `Allow`; el mismo invariante mapa + lista se escribe en dos formas distintas en el mismo archivo
   Fix: añadir `storeLocked(entry *memEntry)` hermano de `removeLocked` (`m.buckets[entry.key] = entry` + `m.pushBackLocked(entry)`, "Caller holds m.mu") y que `Allow` lo llame tras construir la entrada con `key: key`
   Status: done
   Argument: `storeLocked(entry)` hermano de `removeLocked`; `Allow` lo llama tras construir la entrada con `key: key`.
3. [hard] Name for the scope — `windowcounter/limiter.go:506` — en el bucle de lotes de `flushPendingLocked`, `values` son las ranuras de respuesta por clave (el comentario y `parseFlushSlot(values[i])` las llaman slot), `err` es el error de todo el lote (incluido el sintético `RedisIssue` por recuento de ranuras distinto) y convive con `slotErr` y `firstErr`, y `n` es el recuento que Redis devolvió; el lector descodifica qué error es cada uno
   Fix: renombrar a `slots`, `batchErr` y `redisCount` (`slotErr := batchErr`, `redisCount, slotErr = parseFlushSlot(slots[i])`)
   Status: done
   Argument: Renombrados a `slots`, `batchErr` y `redisCount` dentro del nuevo `flushBatchLocked`.
4. [hard] Linear coding — `windowcounter/limiter.go:500` — `flushPendingLocked` (snapshot de pendientes, envío, poda de ventanas, registro de outage) deja inline un segundo trabajo autocontenido nuevo: enviar un lote y aplicar su resultado (límites del lote, EVAL, comprobación de una ranura por clave, parseo por ranura, aplicación y primer error), con un bucle anidado dentro del bucle de lotes
   Fix: extraer `flushBatchLocked(ctx context.Context, batch []pendingFlush) error` que haga el EVAL, la comprobación de ranuras y `applyFlushOutcomeLocked` por clave y devuelva el primer error del lote; el bucle externo solo corta `pending` y conserva `firstErr`
   Status: done
   Argument: Extraído `flushBatchLocked(ctx, batch) error`; el bucle de `flushPendingLocked` solo corta `pending` y conserva `firstErr`.
   Quote:
   ```go
   for batchStart := 0; batchStart < len(pending); batchStart += flushBatchKeys {
   	batchEnd := batchStart + flushBatchKeys
   	if batchEnd > len(pending) {
   		batchEnd = len(pending)
   	}
   	batch := pending[batchStart:batchEnd]
   	values, err := l.flushBatchEvalUnlocked(ctx, batch)
   	// A reply without one slot per key cannot be matched to keys, so every key of this batch fails.
   	if err == nil && len(values) != len(batch) {
   		err = errors.New(simpleredis.RedisIssue)
   	}
   	for i, job := range batch {
   		slotErr := err
   		n := int64(0)
   		if slotErr == nil {
   			n, slotErr = parseFlushSlot(values[i])
   		}
   		if slotErr != nil && firstErr == nil {
   			firstErr = slotErr
   		}
   		l.applyFlushOutcomeLocked(job, n, slotErr)
   	}
   }
   ```
5. [hard] Name for the scope — `windowcounter/limiter.go:538` — el parámetro nuevo `n` de `applyFlushOutcomeLocked` es el recuento que Redis devolvió para la ventana y se asigna a `state.redisKnown`; `n` es un marcador que obliga a leer el comentario para saber qué número es
   Fix: renombrar el parámetro a `redisCount` (`state.redisKnown = redisCount`)
   Status: done
   Argument: Parámetro renombrado a `redisCount` en `applyFlushOutcomeLocked`.
