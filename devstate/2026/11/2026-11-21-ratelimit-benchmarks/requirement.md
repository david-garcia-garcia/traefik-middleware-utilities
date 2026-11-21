# Requirement
IssueKey: 2026-11-21-ratelimit-benchmarks

Fuente: `ticket/source.md` (issueHost local, especificación del solicitante). Dest HEAD analizado: `origin/master` @ `84be0f00`.

## Problem

Los middlewares que usan este repo se ejecutan interpretados por Yaegi (plugin de Traefik) y también compilados dentro de un Traefik propio. Hoy no hay ninguna medida de rendimiento de los dos rate limiters (`tokenbucket/`, `windowcounter/`) en ninguna de las dos modalidades, y el README no explica cómo se comparan entre sí ni frente a los rate limiters de Traefik.

## Current (code)

- Benchmarks de `tokenbucket/`: not found (ningún `func Benchmark` en `tokenbucket/`).
- Benchmarks de `windowcounter/`: not found (ningún `func Benchmark` en `windowcounter/`).
- Patrón compilado + Yaegi existente: `simpleredis/interpretedcost_test.go` (`BenchmarkYaegiGet`: GOPATH temporal con las fuentes no-test, paquete probe con un bucle `GetLoop(host, count)`, un `Eval` por `b.N`, contra el mismo servidor falso que `BenchmarkGet` en `simpleredis/bench_test.go`).
- `tokenbucket.NewMemory`: `tokenbucket/memory.go` — mapa en proceso protegido por `sync.Mutex`, tope de 65536 fuentes (`maxMemorySources`), cero viajes a Redis.
- `tokenbucket.NewRedis`: `tokenbucket/redis.go` — un `Eval` (EVALSHA, con caída a EVAL) del script `allowScript` por `Allow`; el estado vive en un hash de Redis.
- Lua de Traefik: `tokenbucket/lua.go` — copia de `traefik/traefik pkg/middlewares/ratelimiter/lua.go @903e8a9` con dos cambios declarados (`table.maxn` → `#rl_source == 4`; bucket vacío siembra `last = t` en lugar de `0`).
- Redis falso de tokenbucket: `tokenbucket/fake_redis_test.go` — servidor RESP TCP en proceso; responde siempre `NOSCRIPT` a EVALSHA, así que cada `Eval` cuesta dos viajes (EVALSHA + EVAL).
- `windowcounter` con `sync_rate` 0: `windowcounter/limiter.go` (`takeExact`) — por `Take`: un `Eval` (`takeExactScript`, INCR + EXPIRE si no hay TTL) y un `GET` de la ventana anterior.
- `windowcounter` con `sync_rate` > 0: `windowcounter/limiter.go` (`takeBuffered`, `New`) — contador local; `GET` al ver una clave por primera vez y como mucho uno por `sync_rate` para la ventana anterior; un ticker en segundo plano hace el flush (`flushScript`); mínimo 20 ms (`minSyncRate`).
- Redis falso de windowcounter: `windowcounter/fake_redis_test.go` — GET/INCR/EXPIRE/EVAL; también responde siempre `NOSCRIPT` a EVALSHA.
- Arnés Yaegi de los limitadores: `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go` (tests, no benchmarks).
- Variables live: `tokenbucket/limiter_e2e_test.go` (`TOKENBUCKET_LIVE_REDIS`, `TOKENBUCKET_LIVE_DRAGONFLY`), `windowcounter/limiter_e2e_test.go` (`WINDOWCOUNTER_LIVE_REDIS`, `WINDOWCOUNTER_LIVE_DRAGONFLY`).
- README: `README.md` — secciones `## Window counter` y `## Token bucket` describen uso y `sync_rate`; no hay comparativa, ni tabla de benchmarks, ni mención a Traefik Hub `DistributedRateLimit`.
- README sobre modalidad de ejecución: `README.md` `## Yaegi` dice "This code is interpreted inside Traefik, not compiled into it".
- Devdocs de rendimiento: `knowledge/devdocs/std_go_simpleredis.md` — "production is compiled into Traefik, and Yaegi is a portability and e2e-cost concern, not the speed target".
- CI: `.github/workflows/ci.yml` ejecuta `go test` sin `-bench`; los benchmarks solo compilan, no corren.
- Módulo: `go.mod` — `go 1.21`, única dependencia `github.com/traefik/yaegi v0.16.1`.
- `backendbackoff/`: `backendbackoff/bench_test.go` existe (`BenchmarkAllowWarm`); no es un rate limiter (`README.md` `## Backend backoff`).

## Desired

1. Benchmarks Go para:
   - `tokenbucket.NewMemory` y `tokenbucket.NewRedis` contra el Redis falso en proceso; opcionalmente contra Redis/Dragonfly real cuando `TOKENBUCKET_LIVE_*` esté definida.
   - `windowcounter` con `sync_rate` 0 y con `sync_rate` > 0 (opcionalmente live con `WINDOWCOUNTER_LIVE_*`, por simetría con la petición de tokenbucket — ver Tensions).
   - Cada uno en versión compilada y en versión interpretada bajo Yaegi, siguiendo el patrón de `BenchmarkYaegiGet`.
2. Sección del README que explique y compare cada rate limiter: algoritmo, dónde vive el estado, viajes a Redis por petición, ventajas y desventajas, escalabilidad (varias instancias de Traefik, número de claves, caída de Redis), y una tabla de benchmarks obtenidos de este código con comando, máquina y versión de Go.
3. Comparativa cualitativa, solo con hechos documentados, frente a Traefik Proxy `RateLimit` (OSS) y Traefik Hub `DistributedRateLimit` (datos de Hub dados en `ticket/source.md`, leídos el 2026-11-21).

## Affected

- `tokenbucket/` (nuevos ficheros `_test.go` de benchmark).
- `windowcounter/` (nuevos ficheros `_test.go` de benchmark).
- `README.md` (nueva sección comparativa + tabla; posible ajuste de `## Yaegi`, ver Tensions).
- Posiblemente `knowledge/devdocs/std_go_tokenbucket.md`, `knowledge/devdocs/std_go_windowcounter.md`, `knowledge/devdocs/std_go_test-suites.md` (cómo reproducir los benchmarks) — a decidir en explore.

## Out of scope

- Emular la semántica de Traefik Hub (`denyOnError`, `limit` 0, cabeceras `X-RateLimit-*`, `sourceCriterion`) — exclusión explícita del solicitante.
- Cambiar la API pública de `tokenbucket/` o `windowcounter/` — exclusión explícita.
- Añadir dependencias `traefik` o `go-redis` al módulo para benchmarkear contra Traefik — exclusión explícita.
- Tratar `backendbackoff/` como rate limiter; solo puede mencionarse para evitar confusión.
- Inferido, no pedido: ejecutar benchmarks en CI, umbrales de regresión de rendimiento, optimizar los limitadores a raíz de los resultados, benchmarks de `Peek`.

## Unknowns

- Valores reales (ns/op, B/op, allocs/op) en ambas modalidades: sin medir. Máquina de referencia y versión de Go para la tabla: sin fijar (la máquina local reporta `go1.25.6 windows/amd64`; `go.mod` declara `go 1.21`).
- Fidelidad del Redis falso: siempre responde `NOSCRIPT`, así que `NewRedis` y `sync_rate` 0 medidos contra él incluyen un viaje extra por llamada que Redis real no tiene. Si se corrige en el falso, se mide de otra forma o se documenta: lo decide explore.
- Medición de `sync_rate` > 0: el flush ocurre en un ticker en segundo plano; cómo aislar el coste por `Take` frente al flush, y qué valor de `sync_rate` usar, no está definido.
- Representatividad de Yaegi: el módulo fija `yaegi v0.16.1`; la versión de Yaegi embebida en el Traefik objetivo (los tests Pester usan Traefik v3.7.11) no está verificada.
- Forma de los benchmarks live (qué variables, si se saltan sin ellas, paralelismo) y si el benchmark Yaegi live es necesario.
- Patrón de claves (una clave caliente frente a muchas claves) para reflejar "escalabilidad por número de claves" en la tabla.
- Hechos públicos de Traefik Proxy `RateLimit` (OSS) a citar en la comparativa (store en memoria vs Redis, `average`/`period`/`burst`, comportamiento ante caída de Redis): no verificados en esta fase; explore los investiga.
- Hechos de Traefik Hub: solo lo que aporta el solicitante; no verificados contra la doc por este agente.

## Tensions

- Ticket vs README: el ticket afirma que los middlewares también se compilarán dentro de un Traefik propio; `README.md` `## Yaegi` dice "interpreted inside Traefik, not compiled into it". El devdoc `knowledge/devdocs/std_go_simpleredis.md` dice lo contrario ("production is compiled into Traefik"). La nueva sección deberá ser coherente con alguna de las tres afirmaciones.
- Ticket vs código: el ticket dice que `tokenbucket/lua.go` "es copia" del Lua de Traefik; el fichero declara dos modificaciones (`#rl_source == 4` por Dragonfly; sembrado `last = t` en lugar de `0`). La comparativa no puede decir "idéntico".
- Ticket vs código: el ticket pide medir `NewRedis` "contra el Redis falso"; ese falso fuerza EVALSHA → NOSCRIPT → EVAL en cada llamada, así que el número no representa el coste por petición frente a un Redis real con caché de scripts.
- Ticket asimétrico: pide live opcional solo para `tokenbucket/`; para `windowcounter/` no lo menciona, aunque `WINDOWCOUNTER_LIVE_*` existe.
- Ticket vs CI: la tabla debe ser reproducible, pero CI no ejecuta `-bench`; los números del README quedarán fijados a una máquina y fecha, sin verificación automática.
