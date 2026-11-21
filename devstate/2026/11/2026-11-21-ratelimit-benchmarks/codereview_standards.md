# Standards

1. [hard] Symmetry and consistency — `tokenbucket/memory.go:106` — el cambio sustituye `dropExpired`/`dropOne` por la lista de caducidad, pero el hermano `backendbackoff/allow.go:96-145` (`Gate.loadEntry`, `dropExpired`, `dropOne`), copiado de esta misma forma de `tokenbucket/memory.go` según `openspec/changes/archive/2026-09-12-backendbackoff-gate/tasks.md` 1.1, conserva el recorrido O(n) del mapa bajo el mutex. Tiene el mismo tope de 65536 y fija `expireAt = now + cfg.TTL` en cada uso. Las dos formas divergen y la divergencia no queda registrada en ningún sitio (`knowledge/debt/`, `devstate/.../issues.md`).
   Fix: Aplicar la misma lista de caducidad (`key`/`prev`/`next`, `head`/`tail`, `makeRoomLocked`) a `backendbackoff.Gate`. Si el ask excluye `backendbackoff/`, añadir una nota de deuda que diga que el admitido O(n) sigue allí.
   Status: done
   Argument: Aplicada la segunda rama del Fix: nota de deuda `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md` y fila `note large` en `issues.md`; portar la lista a `backendbackoff.Gate` queda fuera del diff (no se aplica unattended).
2. [hard] One job, one owner — `windowcounter/limiter.go:573` — `parseFlushSlot` copia el análisis de entero y el envoltorio `redis:issue?` que ya hace `parseEvalInt` (`limiter.go:613-615`). Si cambia la regla "Flush EVAL integer parse keeps the cause", hay que editar los dos sitios.
   Quote:
   ```go
   n, err := strconv.ParseInt(text, 10, 64)
   if err != nil {
       return 0, fmt.Errorf("%s: %w", simpleredis.RedisIssue, err)
   }
   ```
   Fix: Extraer un único analizador de un slot entero (por ejemplo `parseEvalIntSlot(raw []byte)`) y llamarlo desde `parseEvalInt` y desde la rama entera de `parseFlushSlot`.
   Status: done
   Argument: `parseEvalIntSlot(slot string)` nuevo en `windowcounter/limiter.go`; `parseEvalInt` y la rama entera de `parseFlushSlot` lo llaman.
3. [hard] Name for the scope — `windowcounter/fake_redis_test.go:47` — el campo `flushEvalKeys []int` se llama como si guardara claves, pero guarda el número de claves de cada EVAL de flush. Su propio accesor ya lo llama `flushEvalKeyCounts()`.
   Fix: Renombrar el campo a `flushEvalKeyCounts`.
   Status: done
   Argument: Campo renombrado a `flushBatchKeyCounts`: Go no permite un campo y un método con el mismo nombre (`flushEvalKeyCounts()` ya existe), así que el accesor conserva su nombre.
4. [judgement] Duplicated Code — `windowcounter/fake_redis_test.go:40` — la caché opcional de scripts (`scriptCache`, `loadedScripts`, la reescritura EVALSHA→EVAL, `enableScriptCache`), el contador `commands`/`commandCount`, `benchKeyNames` y `reportRedisCmdsPerOp` están copiados casi literalmente en `tokenbucket/fake_redis_test.go:29` y `tokenbucket/bench_test.go:23,73`. El arnés de benchmarks Yaegi se repite con otra forma: es el helper `runYaegiAllowBench` en `tokenbucket/bench_yaegi_test.go:13` y va escrito en línea dentro de `BenchmarkYaegiTake` en `windowcounter/bench_yaegi_test.go:13`. Así se amplía la copia por paquete que ya existía (`copyNonTestGo`, `callerDir`, `liveEngineAddrs`, `waitLiveClient`), y un arreglo en el fake o en el arnés hay que replicarlo a mano.
   Fix: Aceptar la copia por paquete de forma explícita (anotarla en `std_go_test-suites.md`) o pasar el arnés común a un paquete compartido solo de test. Como mínimo, dar a los dos benchmarks Yaegi la misma forma (helper en ambos o en línea en ambos).
   Status: skipped
   Argument: judgement: la copia por paquete del arnés de test es la forma vigente del repo (`copyNonTestGo`, `liveEngineAddrs`); unificarla en un paquete compartido es un cambio de estructura de tests fuera de este ticket.
