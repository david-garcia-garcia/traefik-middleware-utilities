# Requirement
IssueKey: 2026-11-21-ratelimit-benchmarks

Fuente: `ticket/source.md` completo (issueHost local, especificación del solicitante), incluida la sección `## Ampliación de alcance (2026-11-21)`. Dest HEAD analizado: `origin/master` @ `84be0f00`. PR #105 sin comentarios abiertos.

## Problem

Los middlewares que usan este repo se ejecutan interpretados por Yaegi (plugin de Traefik) y también compilados dentro de un Traefik propio. Hoy no hay ninguna medida de rendimiento de los dos rate limiters (`tokenbucket/`, `windowcounter/`) en ninguna de las dos modalidades, y el README no explica cómo se comparan entre sí ni frente a los rate limiters de Traefik.

Ampliación (2026-11-21): dos costes crecen con el número de claves y el solicitante quiere reducirlos en este mismo cambio sin alterar el comportamiento observable: el alta de una fuente nueva en `tokenbucket.Memory` recorre todo el mapa bajo el mutex, y el flush de `windowcounter` con `sync_rate` > 0 envía un EVAL por clave en serie. Además, un devdoc afirma que Yaegi no es objetivo de velocidad, lo que contradice que el plugin se ejecute también bajo Yaegi.

## Current (code)

- Benchmarks de `tokenbucket/`: not found (ningún `func Benchmark` en `tokenbucket/`).
- Benchmarks de `windowcounter/`: not found (ningún `func Benchmark` en `windowcounter/`).
- Patrón compilado + Yaegi existente: `simpleredis/interpretedcost_test.go` (`BenchmarkYaegiGet`: GOPATH temporal con las fuentes no-test, paquete probe con un bucle `GetLoop(host, count)`, un `Eval` por `b.N`, contra el mismo servidor falso que `BenchmarkGet` en `simpleredis/bench_test.go`).
- `tokenbucket.NewMemory`: `tokenbucket/memory.go` — mapa en proceso protegido por un único `sync.Mutex`, tope de 65536 fuentes (`maxMemorySources`), cero viajes a Redis.
- Alta de fuente nueva en `tokenbucket.Memory`: `tokenbucket/memory.go` (`Allow` → `dropExpired`) — con el mutex tomado, cada clave ausente o caducada recorre el mapa entero borrando las entradas con `expireAt` vencido; la clave existente y vigente no recorre el mapa.
- Tope de fuentes en `tokenbucket.Memory`: `tokenbucket/memory.go` (`dropOne`) — al llegar a 65536 tras purgar caducadas, borra una entrada cualquiera (primera del orden de iteración del mapa, no determinista).
- Tests de `tokenbucket.Memory` que fijan TTL y ráfaga: `tokenbucket/limiter_test.go` (`TestMemory_IdlePastTTLStartsFull`, `TestMemory_BurstAfterIdle`, `TestMemory_RefundWhenWaitExceedsMaxDelay`, `TestMemoryAndRedis_Agree`). Test del tope 65536 / desalojo: not found.
- `tokenbucket.NewRedis`: `tokenbucket/redis.go` — un `Eval` (EVALSHA, con caída a EVAL) del script `allowScript` por `Allow`; el estado vive en un hash de Redis.
- Lua de Traefik: `tokenbucket/lua.go` — copia de `traefik/traefik pkg/middlewares/ratelimiter/lua.go @903e8a9` con dos cambios declarados (`table.maxn` → `#rl_source == 4`; bucket vacío siembra `last = t` en lugar de `0`).
- Redis falso de tokenbucket: `tokenbucket/fake_redis_test.go` — servidor RESP TCP en proceso; responde siempre `NOSCRIPT` a EVALSHA, así que cada `Eval` cuesta dos viajes (EVALSHA + EVAL).
- `windowcounter` con `sync_rate` 0: `windowcounter/limiter.go` (`takeExact`) — por `Take`: un `Eval` (`takeExactScript`, INCR + EXPIRE si no hay TTL) y un `GET` de la ventana anterior.
- `windowcounter` con `sync_rate` > 0: `windowcounter/limiter.go` (`takeBuffered`, `New`) — contador local; `GET` al ver una clave por primera vez y como mucho uno por `sync_rate` para la ventana anterior; mínimo 20 ms (`minSyncRate`).
- Flush de `windowcounter`: `windowcounter/limiter.go` (`flushPendingLocked`, `flushEvalUnlocked`) — copia bajo `l.mu` las ventanas con `localDelta` > 0, y por cada una suelta el mutex, envía un `Eval` de `flushScript` (INCRBY + EXPIREAT si es nueva; `KEYS` lista la clave por Dragonfly) y lo vuelve a tomar, en serie. Lo disparan `time.AfterFunc` cada `sync_rate` (`startFlushLocked`, `flushAfterTick`), `Sleep` y `Close`.
- Semántica de errores del flush: `windowcounter/limiter.go` (`flushPendingLocked`) — un fallo en una clave solo anula el `flushDelta` de esa clave (el delta se reintenta en el siguiente flush) y no impide las demás; el primer error se guarda en `lastOutageErr` y, durante un `sync_rate`, `Take`/`Peek` en modo buffered no contactan Redis y admiten con el límite local y error nil (`skipRedisContactLocked`). Un flush correcto borra el error guardado.
- Tests que fijan esa semántica: `windowcounter/repro_buffered_outage_test.go`, `windowcounter/limiter_test.go`, `windowcounter/repro_stale_previous_test.go`, `windowcounter/yaegi_flush_stop_test.go`.
- Restricción Yaegi del flush: `windowcounter/limiter.go` (comentario de `startFlushLocked`) y `knowledge/devdocs/std_go_windowcounter.md` — `time.AfterFunc` en lugar de `go` + `select`, porque Yaegi v0.16.1 puede no ver el cierre y colgar `Sleep`/`Close`.
- Redis falso de windowcounter: `windowcounter/fake_redis_test.go` — GET/INCR/EXPIRE/EVAL; también responde siempre `NOSCRIPT` a EVALSHA.
- Arnés Yaegi de los limitadores: `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go` (tests, no benchmarks).
- Variables live: `tokenbucket/limiter_e2e_test.go` (`TOKENBUCKET_LIVE_REDIS`, `TOKENBUCKET_LIVE_DRAGONFLY`), `windowcounter/limiter_e2e_test.go` (`WINDOWCOUNTER_LIVE_REDIS`, `WINDOWCOUNTER_LIVE_DRAGONFLY`).
- README: `README.md` — secciones `## Window counter` y `## Token bucket` describen uso y `sync_rate`; no hay comparativa, ni tabla de benchmarks, ni mención a Traefik Hub `DistributedRateLimit`.
- README sobre modalidad de ejecución: `README.md` (introducción: "It does not compile your plugin into Traefik"; `## Yaegi`: "This code is interpreted inside Traefik, not compiled into it").
- Frase de devdoc a corregir: `knowledge/devdocs/std_go_simpleredis.md` — "production is compiled into Traefik, and Yaegi is a portability and e2e-cost concern, not the speed target".
- CI: `.github/workflows/ci.yml` ejecuta `go test` sin `-bench`; los benchmarks solo compilan, no corren.
- Módulo: `go.mod` — `go 1.21`, única dependencia `github.com/traefik/yaegi v0.16.1`.
- `backendbackoff/`: `backendbackoff/bench_test.go` existe (`BenchmarkAllowWarm`); no es un rate limiter (`README.md` `## Backend backoff`).

## Desired

1. Benchmarks Go para:
   - `tokenbucket.NewMemory` y `tokenbucket.NewRedis` contra el Redis falso en proceso; opcionalmente contra Redis/Dragonfly real cuando `TOKENBUCKET_LIVE_*` esté definida.
   - `windowcounter` con `sync_rate` 0 y con `sync_rate` > 0 (opcionalmente live con `WINDOWCOUNTER_LIVE_*`, por simetría con la petición de tokenbucket — ver Tensions).
   - Cada uno en versión compilada y en versión interpretada bajo Yaegi, siguiendo el patrón de `BenchmarkYaegiGet`.
   - Los benchmarks deben mostrar el antes y el después de las dos optimizaciones del punto 4.
2. Sección del README que explique y compare cada rate limiter: algoritmo, dónde vive el estado, viajes a Redis por petición, ventajas y desventajas, escalabilidad (varias instancias de Traefik, número de claves, caída de Redis), y una tabla de benchmarks obtenidos de este código con comando, máquina y versión de Go. Máquina de referencia: Core Ultra 7 265K, Windows 11, go1.25.6, `-count 5`, mediana.
3. Comparativa cualitativa, solo con hechos documentados, frente a Traefik Proxy `RateLimit` (OSS) y Traefik Hub `DistributedRateLimit` (datos de Hub dados en `ticket/source.md`, leídos el 2026-11-21).
4. Optimizar en este cambio los dos costes que crecen con el número de claves:
   - (a) el recorrido completo del mapa bajo el mutex en cada fuente nueva de `tokenbucket.Memory` (`dropExpired`);
   - (b) el EVAL por clave en serie del flush de `windowcounter` con `sync_rate` > 0.
   Restricciones de ambas: mismo comportamiento observable (mismas decisiones de admitir/denegar, mismos topes, misma semántica ante errores y caída de Redis), seguras bajo Yaegi, sin dependencias nuevas.
5. Corregir la frase de `knowledge/devdocs/std_go_simpleredis.md` que dice que Yaegi no es objetivo de velocidad: el plugin se ejecuta compilado y bajo Yaegi.

## Affected

- `tokenbucket/` (nuevos ficheros `_test.go` de benchmark; `memory.go` por el punto 4a).
- `windowcounter/` (nuevos ficheros `_test.go` de benchmark; `limiter.go` por el punto 4b).
- `README.md` (nueva sección comparativa + tabla; posible ajuste de la introducción y de `## Yaegi`, ver Tensions).
- `knowledge/devdocs/std_go_simpleredis.md` (frase del punto 5).
- Posiblemente `knowledge/devdocs/std_go_tokenbucket.md`, `knowledge/devdocs/std_go_windowcounter.md`, `knowledge/devdocs/std_go_test-suites.md` (cómo reproducir los benchmarks y qué se optimizó) — a decidir en explore/propose.

## Out of scope

- Emular la semántica de Traefik Hub (`denyOnError`, `limit` 0, cabeceras `X-RateLimit-*`, `sourceCriterion`) — exclusión explícita del solicitante.
- Cambiar la API pública de `tokenbucket/` o `windowcounter/` — exclusión explícita.
- Añadir dependencias `traefik` o `go-redis` al módulo para benchmarkear contra Traefik — exclusión explícita. Ninguna dependencia nueva en general (ampliación).
- Tratar `backendbackoff/` como rate limiter; solo puede mencionarse para evitar confusión.
- Inferido, no pedido: ejecutar benchmarks en CI, umbrales de regresión de rendimiento, benchmarks de `Peek`, otras optimizaciones de los limitadores distintas de las dos del punto 4 (p. ej. el coste por petición de `NewRedis` o de `sync_rate` 0), cambiar el tope 65536 o el TTL, cambiar `simpleredis/` salvo que el punto 4b lo exija (ver Unknowns).

## Unknowns

- Valores reales (ns/op, B/op, allocs/op) en ambas modalidades, antes y después: sin medir en este documento.
- Fidelidad del Redis falso: siempre responde `NOSCRIPT`, así que `NewRedis`, `sync_rate` 0 y el flush medidos contra él incluyen un viaje extra por llamada que Redis real no tiene. Si se corrige en el falso, se mide de otra forma o se documenta: lo decide explore.
- Medición de `sync_rate` > 0: el flush ocurre en `time.AfterFunc` en segundo plano; cómo aislar el coste por `Take` frente al flush, cómo medir el flush en sí con muchas claves, y qué valor de `sync_rate` usar, no está definido.
- Cómo se consigue cada optimización (estructura para la caducidad en 4a; forma de agrupar el flush en 4b) y si 4b necesita algo de `simpleredis/` que hoy no expone: no decidido aquí.
- Qué cuenta como "misma semántica ante errores" en 4b cuando varias claves viajan juntas: hoy el fallo es por clave (`flushPendingLocked`); la equivalencia con un envío agrupado no está definida.
- Qué cuenta como "mismos topes" en 4a: el desalojo al llegar a 65536 hoy borra una entrada arbitraria del mapa (`dropOne`); si otra elección de víctima sigue siendo "el mismo tope" no está definido.
- Compatibilidad Dragonfly de un flush que toque varias claves: `flushScript` ya declara las claves en `KEYS` por Dragonfly; el comportamiento con varias claves por llamada no está verificado.
- Representatividad de Yaegi: el módulo fija `yaegi v0.16.1`; la versión de Yaegi embebida en el Traefik objetivo (los tests Pester usan Traefik v3.7.11) no está verificada en esta fase.
- Forma de los benchmarks live (qué variables, si se saltan sin ellas, paralelismo) y si el benchmark Yaegi live es necesario.
- Patrón de claves (una clave caliente frente a muchas claves; cuántas) para reflejar "escalabilidad por número de claves" y el antes/después en la tabla.
- Cómo presenta la tabla el "antes" (commit de referencia, columnas) una vez la optimización esté en el código: no definido.
- Hechos públicos de Traefik Proxy `RateLimit` (OSS) a citar en la comparativa: los investiga explore.
- Hechos de Traefik Hub: solo lo que aporta el solicitante; no verificados contra la doc por este agente.

## Tensions

- Ticket vs README vs devdoc: el ticket afirma que los middlewares también se compilarán dentro de un Traefik propio y que el plugin se ejecuta compilado y bajo Yaegi; `README.md` (introducción y `## Yaegi`) dice que se interpreta y no se compila; `knowledge/devdocs/std_go_simpleredis.md` dice "production is compiled into Traefik". La ampliación ordena corregir solo la frase del devdoc; el README no se nombra, así que la nueva sección del README y las frases existentes pueden quedar incoherentes.
- Ticket vs código: el ticket dice que `tokenbucket/lua.go` "es copia" del Lua de Traefik; el fichero declara dos modificaciones. La comparativa no puede decir "idéntico".
- Ticket vs código: el ticket pide medir `NewRedis` "contra el Redis falso"; ese falso fuerza EVALSHA → NOSCRIPT → EVAL en cada llamada, así que el número no representa el coste frente a un Redis real con caché de scripts. Lo mismo afecta al antes/después del flush (4b).
- Ticket asimétrico: pide live opcional solo para `tokenbucket/`; para `windowcounter/` no lo menciona, aunque `WINDOWCOUNTER_LIVE_*` existe.
- "Comportamiento observable no debe cambiar" vs código: el desalojo al tope de `tokenbucket.Memory` no es determinista hoy y no hay test que lo fije; el fallo del flush es por clave. Ambas cosas acotan qué cambios de 4a/4b cumplen la restricción.
- Máquina de referencia vs módulo: la tabla se fija en go1.25.6; `go.mod` declara `go 1.21`. Los números no representan la versión mínima declarada.
- Ticket vs CI: la tabla debe ser reproducible, pero CI no ejecuta `-bench`; los números del README quedarán fijados a una máquina y fecha, sin verificación automática.
