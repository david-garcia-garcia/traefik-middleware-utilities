# Issues

- [x] take large  `knowledge/debt/2026-11-21-tokenbucket-memory-new-source-scan.md` → `tokenbucket/memory.go` alta O(1) con lista de caducidad
  Why: `tokenbucket.Memory` recorre todos los buckets bajo su mutex en cada fuente nueva (≈0.8–1.6 ms con 60k–65k fuentes). El solicitante lo metió en este cambio el 2026-11-21 (respuesta 3). Cuando implement lo resuelva: `[x]`, `Taken:` y borrar el fichero de deuda.
  Taken: implement, commits `ae7f4a9a` y `a51d20a3` — lista intrusiva ordenada por caducidad en `tokenbucket/memory.go` (alta O(1), tope 65536, caducadas primero); fichero de deuda borrado.
- [x] take large  `knowledge/debt/2026-11-21-windowcounter-serial-flush.md` → `windowcounter/limiter.go` flush con EVAL por lotes
  Why: el flush buffered envía un EVAL por clave sucia en serie, así que el intervalo real de sincronización crece con el número de claves. El solicitante lo metió en este cambio el 2026-11-21 (respuesta 3). Cuando implement lo resuelva: `[x]`, `Taken:` y borrar el fichero de deuda.
  Taken: implement, commit `ae7f4a9a` — flush por lotes de 256 claves por EVAL con `redis.pcall` y resultado por clave en `windowcounter/limiter.go`; fichero de deuda borrado.
- [ ] note large  `knowledge/debt/2026-11-21-simpleredis-spec-compiled-only-rationale.md`
  Why: el spec vivo `std_go_simpleredis_resp-commands` sigue diciendo que el despliegue es solo compilado dentro de Traefik; este cambio corrige esa premisa en el README y en `std_go_simpleredis.md`, pero no en el spec.
- [ ] note large  `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`
  Why: `backendbackoff.Gate` sigue recorriendo todo su mapa bajo el mutex en cada clave nueva con la tabla llena; este cambio solo pasa `tokenbucket.Memory` a la lista de caducidad O(1) y los dos hermanos divergen.
