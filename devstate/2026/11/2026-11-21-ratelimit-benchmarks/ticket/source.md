# Benchmarks de los rate limiters (compilado y Yaegi) y comparativa en el README

issueHost: local · issueRef: none · fuente: especificación del solicitante pasada por el llamador (2026-11-21).

Contexto: los middlewares construidos sobre este repo se cargan como plugin de Traefik (interpretado por Yaegi) y TAMBIÉN se compilarán dentro de un Traefik propio. Por eso cualquier medida de rendimiento debe darse en las dos modalidades.

Petición:
1. Añadir benchmarks Go en el código para los rate limiters del repo:
   - `tokenbucket/`: `NewMemory` y `NewRedis` (contra el Redis falso en proceso que ya usan los tests; opcionalmente contra Redis/Dragonfly real cuando las variables `*_LIVE_*` estén definidas).
   - `windowcounter/`: con `sync_rate` 0 (Redis en cada llamada) y con `sync_rate` > 0 (lotes locales).
   - Cada uno en versión compilada y en versión interpretada bajo Yaegi (el repo ya tiene el patrón en `simpleredis/interpretedcost_test.go`, `BenchmarkYaegiGet`).
2. Mejorar el README con una sección que explique y compare el funcionamiento de cada rate limiter: algoritmo, dónde vive el estado, viajes a Redis por petición, ventajas y desventajas, escalabilidad (varias instancias de Traefik, número de claves, caída de Redis), y una tabla de benchmarks que provengan de nuestro código, indicando comando, máquina y versión de Go para que sean reproducibles.
3. La comparativa puede situar nuestros limitadores frente a Traefik Proxy `RateLimit` (OSS; `tokenbucket/lua.go` es copia de su Lua) y Traefik Hub `DistributedRateLimit`, de forma cualitativa y solo con hechos documentados. Datos de la doc de Hub (leída el 2026-11-21): "basado en token bucket"; opciones `limit` (0 = sin límite), `period` (1s), `burst` (1), `denyOnError` (true), `responseHeaders` (X-RateLimit-Remaining/Limit/Period/Reset), `store.redis` (single, cluster, sentinel, TLS), `sourceCriterion`. Su implementación es cerrada: no documenta caché local, sincronización, viajes a Redis por petición ni Retry-After.

Fuera de alcance (decisión explícita del solicitante): emular la semántica de Traefik Hub (denyOnError, limit 0, cabeceras X-RateLimit, sourceCriterion) o cambiar la API pública de `tokenbucket/` / `windowcounter/`. No añadir dependencias de `traefik` ni `go-redis` al módulo para benchmarkear contra Traefik. `backendbackoff/` no es un rate limiter; puede mencionarse solo para evitar confusión.
