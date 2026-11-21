# Issues

- [ ] take large  `knowledge/debt/2026-11-21-tokenbucket-memory-new-source-scan.md` → `tokenbucket/memory.go` alta O(1) con lista de caducidad
  Why: `tokenbucket.Memory` recorre todos los buckets bajo su mutex en cada fuente nueva (≈0.8–1.6 ms con 60k–65k fuentes). El solicitante lo metió en este cambio el 2026-11-21 (respuesta 3). Cuando implement lo resuelva: `[x]`, `Taken:` y borrar el fichero de deuda.
- [ ] take large  `knowledge/debt/2026-11-21-windowcounter-serial-flush.md` → `windowcounter/limiter.go` flush con EVAL por lotes
  Why: el flush buffered envía un EVAL por clave sucia en serie, así que el intervalo real de sincronización crece con el número de claves. El solicitante lo metió en este cambio el 2026-11-21 (respuesta 3). Cuando implement lo resuelva: `[x]`, `Taken:` y borrar el fichero de deuda.
