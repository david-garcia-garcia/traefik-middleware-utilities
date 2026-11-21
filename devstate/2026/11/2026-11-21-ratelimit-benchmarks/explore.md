# Explore

## Concepts

### Unidades que toca este cambio

| Unidad | Ruta | Papel en este cambio |
|---|---|---|
| Token bucket en memoria | `tokenbucket/memory.go` (`Memory.Allow`) | Se mide y se optimiza: mapa en proceso bajo un `sync.Mutex`, tope `maxMemorySources` = 65536; quitar el recorrido O(n) en cada alta (`dropExpired`, `dropOne`) |
| Token bucket Redis | `tokenbucket/redis.go` (`Redis.Allow`) | Se mide: un `simpleredis.Eval` (EVALSHA y, si responde NOSCRIPT, EVAL) del `allowScript` por llamada |
| Window counter | `windowcounter/limiter.go` (`Take`: `takeExact` / `takeBuffered`, `flushPendingLocked`) | Se mide y se optimiza: `sync_rate` 0 = EVAL + GET por Take; `sync_rate` > 0 = memoria con GET ocasional y flush en un `time.AfterFunc`; el flush pasa de un EVAL por clave a lotes |
| Eval de SimpleRedis | `simpleredis/commands_eval.go` (`Eval`) | No cambia. EVALSHA primero; con NOSCRIPT reenvía EVAL. No guarda estado de "script cargado" |
| Redis falso de tokenbucket | `tokenbucket/fake_redis_test.go` (`testFakeRedis`, `startTestFakeRedis(t *testing.T)`) | Test-only. Responde siempre NOSCRIPT a EVALSHA |
| Redis falso de windowcounter | `windowcounter/fake_redis_test.go` (ídem) | Test-only. Responde siempre NOSCRIPT a EVALSHA |
| Redis falso de simpleredis (referencia) | `simpleredis/fake_redis_test.go` (`loadedScripts`) | Precedente en el repo: el falso guarda el digest en cada EVAL y responde a EVALSHA si lo conoce |
| Arnés Yaegi | `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go` (`writeGopath*`, `copyNonTestGo`, `callerDir`, `writeGopathFile`, todos con `t *testing.T`) | Se reutiliza en los benchmarks interpretados |
| Patrón de benchmark Yaegi | `simpleredis/interpretedcost_test.go` `BenchmarkYaegiGet` | Patrón a copiar: GOPATH temporal, paquete probe con `Loop(count)`, un `Eval` de calentamiento y luego un `Eval(b.N)` cronometrado |
| Selección de motor live | `tokenbucket/limiter_e2e_test.go`, `windowcounter/limiter_e2e_test.go` (`lookupLiveEngineAddrs` es pura; `liveEngineAddrs` / `runForEachLiveEngine` usan `*testing.T`) | Se reutiliza `lookupLiveEngineAddrs` para los benchmarks live |
| README | `README.md` `## Window counter`, `## Token bucket`, `## Yaegi`, `## Tests` | Nueva sección comparativa con tabla; se corrige la frase de `## Yaegi` |
| Devdocs | `knowledge/devdocs/std_go_tokenbucket.md`, `std_go_windowcounter.md`, `std_go_test-suites.md`, `std_go_simpleredis.md` | Uso: cómo reproducir los benchmarks; corregir en `std_go_simpleredis.md` la frase de que Yaegi no es objetivo de velocidad |

### Cómo se conectan

```
                 compilado (go test -bench)            interpretado (Yaegi v0.16.1)
                 ─────────────────────────             ────────────────────────────
 Benchmark*  ──► tokenbucket.Memory.Allow              interp.Eval("probe.Loop(N)")
             ──► tokenbucket.Redis.Allow ─┐                 │  (fuentes no-test copiadas a GOPATH)
             ──► windowcounter.Take ──────┤                 ▼
                                          │            mismas funciones, interpretadas
                                          ▼                 │
                              simpleredis.Eval / Get ◄──────┘
                                          │  TCP loopback
                     ┌────────────────────┴──────────────────────┐
                     ▼                                           ▼
       testFakeRedis (por paquete, _test.go)          Redis 7 / Dragonfly real
       hoy: EVALSHA → NOSCRIPT → EVAL (2 viajes)      (TOKENBUCKET_LIVE_*, WINDOWCOUNTER_LIVE_*)
       propuesta: caché opt-in de digests (1 viaje)   EVALSHA acierta tras el primer EVAL

 windowcounter sync_rate > 0:  Take ─► mapa local (mutex) ─► GET solo al ver la clave / tras flush / previo ≤1 por sync_rate
                               time.AfterFunc(sync_rate) ─► flushPendingLocked: un EVAL por clave sucia, en serie
```

### Call sites que importan (raíces buscadas: `tokenbucket/`, `windowcounter/`, `simpleredis/`)

- `func Benchmark` en `tokenbucket/` y `windowcounter/`: 0 (grep). Existentes en el repo: `simpleredis/bench_test.go`, `simpleredis/interpretedcost_test.go`, `backendbackoff/bench_test.go`.
- Helpers test-only que habría que ampliar de `*testing.T` a `testing.TB` para usarlos desde `*testing.B` (cambio compatible: todos los llamadores pasan `*testing.T`): `startTestFakeRedis` 11 usos en `tokenbucket/`, 37 en `windowcounter/`; `writeGopathTokenbucket` 4; `writeGopathWindowcounter` 5; `copyNonTestGo` 3 + 3; `callerDir` 2 + 2; `writeGopathFile` 4 + 6. En el probe de esta fase el cambio de firma compiló sin tocar ningún llamador (`go vet` limpio).
- Tests que leen `lastEvalCommand()` (dependen de que EVAL llegue al falso): 4 en `tokenbucket/` + `windowcounter/`. Por eso la caché de scripts del falso debe ser opt-in y estar apagada por defecto.

### Reproducción

- "No hay benchmarks de los limitadores": reproducido (grep `func Benchmark` sin resultados en `tokenbucket/` ni `windowcounter/`).
- "El falso fuerza dos viajes por Eval": reproducido. Probe en copia temporal del módulo (fuera del worktree, borrable), con métrica `redis-cmds/op` contada en el falso:

  Máquina: Intel Core Ultra 7 265K (20 hilos), Windows 11, `go1.25.6 windows/amd64`, `-benchtime 1s -count 1`. Números de un solo run: orientativos, no son la tabla del README.

  | Probe | ns/op | B/op | allocs/op | redis-cmds/op |
  |---|---|---|---|---|
  | `Memory.Allow` clave caliente | 19 | 0 | 0 | 0 |
  | `Memory.Allow` 10k claves en rotación | 40 | 0 | 0 | 0 |
  | `Memory.Allow` clave caliente, `RunParallel` | 55 | 0 | 0 | 0 |
  | `Memory.Allow` clave nueva con 60k claves vivas | 777 757 | 244 | 2 | 0 |
  | `Memory.Allow` clave nueva con la tabla llena (65536) | 1 635 572 | 62 | 2 | 0 |
  | `Redis.Allow`, falso actual (NOSCRIPT siempre) | 69 517 | 8698 | 125 | 2.0 |
  | `Redis.Allow`, falso con caché de digest | 33 814 | 2370 | 69 | 1.0 |
  | `Redis.Allow`, caché, `RunParallel` | 23 102 | 2630 | 72 | — |
  | Yaegi `Memory.Allow` clave caliente | 11 747 | — | — | 0 |
  | Yaegi `Redis.Allow`, falso con caché | 204 202 | — | — | — |
  | Yaegi `Redis.Allow`, falso actual | 377 665 | — | — | — |
  | `windowcounter` `sync_rate` 0, falso actual | 57 124 | 4468 | 105 | 3.0 |
  | `windowcounter` `sync_rate` 0, falso con caché | 49 910 | 2466 | 64 | 2.0 |
  | `windowcounter` `sync_rate` 1s, clave caliente | 134 | 80 | 4 | 0.0000008 |
  | `windowcounter` `sync_rate` 20ms, clave caliente | 131 | 80 | 4 | 0.000016 |
  | `windowcounter` `sync_rate` 1s, 10k claves | 414 | 97 | 4 | 0.0086 |
  | `windowcounter` `sync_rate` 1s, `RunParallel` | 202 | 80 | 4 | — |
  | Yaegi `windowcounter` `sync_rate` 0, caché | 272 209 | — | — | — |
  | Yaegi `windowcounter` `sync_rate` 1s | 17 817 | — | — | — |
  | `flushPending` directo, 100 / 1000 / 10000 claves sucias | 2.05 ms / 20.6 ms / 211 ms (≈21 µs por clave) | | | |

  Lecturas que salen de estas medidas (inferencias sobre el código, comprobadas con el probe):
  - El falso actual duplica el coste de cada Eval: `Redis.Allow` 69.5 µs frente a 33.8 µs con caché; `redis-cmds/op` 2 frente a 1.
  - `Memory.Allow` con una clave nueva recorre todo el mapa (`dropExpired` en `tokenbucket/memory.go`, llamado en cada alta y otra vez en `dropOne` cuando la tabla está llena), y lo hace bajo el mutex global: unos 0.8 ms con 60k claves y 1.6 ms con la tabla llena, mientras bloquea cualquier otro `Allow`. Las claves ya conocidas cuestan 20–40 ns.
  - En `sync_rate` > 0 el tráfico a Redis crece con claves activas / `sync_rate`, no con peticiones; el coste por Take es casi el mismo con 20 ms o con 1 s.
  - El flush va en serie, un EVAL por clave (`flushPendingLocked`): con N claves sucias dura unas N × RTT. Si N × RTT supera `sync_rate`, el intervalo efectivo de sincronización crece con el número de claves.
  - Yaegi multiplica la ruta en memoria unas 600× (19 ns → 11.7 µs) y la ruta Redis unas 5–6× (34 µs → 204 µs).
  - El Redis falso de cada paquete hace la lectura y la escritura de cada comando bajo un único `f.mu`. Las variantes `RunParallel` contra el falso miden el cliente más un servidor serializado, no la escalabilidad de Redis real.

### Hechos externos usados

- `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (Traefik Proxy OSS `RateLimit`: en memoria usa `golang.org/x/time/rate`, tope de 65536 fuentes; con Redis, EVALSHA/EVAL de `AllowTokenBucketRaw`; `average`/`period`/`burst`; `maxDelay` calculado en Go, no configurable; error de Redis → HTTP 500 en el pin `903e8a9`). Esta fase actualizó el extracto de la documentación (fetched 2026-11-21: opciones Redis documentadas; la doc no dice nada sobre la caída de Redis).
- `knowledge/research/ext_traefik_hub_distributed-rate-limit/` (creado en esta fase, a partir de la doc oficial de Hub leída el 2026-11-21).
- `knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`: Traefik v3.7.11 fija `github.com/traefik/yaegi v0.16.1`, la misma versión que el `go.mod` de este repo.
- `knowledge/research/ext_kong_rate-limiting_sliding-sync/` (semántica de `sync_rate` y suelo de 20 ms).
- En el propio repo: el código citado arriba y el probe.

## Decisions

### Enfoque elegido

- **Ficheros de benchmark por paquete, todos `_test.go`**: nombres por dominio, siguiendo `std_go_test-suites.md` y el precedente `bench_test.go`: `bench_test.go` (compilado, contra el falso), `bench_yaegi_test.go` (interpretado, contra el falso) y `bench_e2e_test.go` (live) en `tokenbucket/` y en `windowcounter/`. El nombre final lo fija propose.
- **Redis falso con caché de scripts opt-in**: se añade a cada `testFakeRedis` un campo test-only que, cuando el benchmark lo activa, guarda el digest en cada EVAL y responde a EVALSHA si lo conoce. Es la misma forma que `simpleredis/fake_redis_test.go` `loadedScripts`. Apagado por defecto, así que los tests existentes (los que leen `lastEvalCommand`) no cambian. Representa a un Redis real con la caché de scripts caliente.
- **Métrica `redis-cmds/op`** (`b.ReportMetric`) en los benchmarks contra el falso. Deja ver en la tabla los viajes por petición y la parte amortizada del modo buffered.
- **Matriz de claves**: clave caliente (`ip:1.2.3.4`) y 10 000 claves en rotación precalentadas. `tokenbucket.Memory` tiene además la variante "clave nueva con la tabla llena (65536)", que muestra el coste de alta y desalojo.
- **Paralelismo**: `b.RunParallel` solo en compilado, en memoria y en Redis. El README dirá que el paralelo contra el falso está limitado por el falso serializado y que el paralelo live es el que vale para hablar de escalabilidad.
- **`windowcounter` con `sync_rate` > 0**: se mide el coste por Take en el bucle `b.N` con el flush real en segundo plano, a `sync_rate` = 20 ms (`minSyncRate`, el caso con más tráfico a Redis), y se reporta `redis-cmds/op`. Aparte, un `BenchmarkFlush` con 1000 claves sucias llama a `flushPending` dentro del paquete, para que el README diga con un número del código que el flush es lineal en claves.
- **Yaegi**: se copia el patrón de `BenchmarkYaegiGet`: un intérprete por benchmark, un paquete probe con `Loop(host, count)` (o `MemoryLoop(count)`), un `Eval` de calentamiento y luego `Eval(b.N)` cronometrado, sin `ReportAllocs`. Los helpers `writeGopath*`, `copyNonTestGo`, `callerDir`, `writeGopathFile` y `startTestFakeRedis` pasan a `testing.TB`. Variantes interpretadas: memoria (clave caliente), Redis/`sync_rate` 0 y `sync_rate` > 0, contra el falso con caché.
- **Live**: benchmarks compilados por motor (`b.Run("redis"|"dragonfly")`) que reutilizan `lookupLiveEngineAddrs`. Se saltan con `-short` o con las dos variables sin definir (las mismas reglas que los tests live). Las claves llevan un prefijo único por ejecución. Existen para `TOKENBUCKET_LIVE_*` y para `WINDOWCOUNTER_LIVE_*`, por simetría. Yaegi live no se hace (ver Rejected).
- **README**: se añade una sección comparativa nueva (algoritmo, dónde vive el estado, viajes a Redis por petición, ventajas e inconvenientes, escalabilidad con varias instancias, número de claves y caída de Redis) con una tabla de benchmarks generada por los benchmarks anteriores, con el comando, la máquina y la versión de Go. La comparativa con Traefik Proxy `RateLimit` y Traefik Hub `DistributedRateLimit` se apoya solo en los dos folders de research. `tokenbucket/lua.go` se describe como "derivado del Lua de Traefik con dos cambios declarados", nunca como "idéntico". `backendbackoff/` solo se menciona para aclarar que no es un rate limiter.
- **Modalidad de ejecución, tres frases a la vez** (respuesta 1 del solicitante y Desired 5; así se cierra la Tension "Ticket vs README vs devdoc"):
  1. Introducción del README (`## Why this exists`, "It does not compile your plugin into Traefik"): se mantiene, porque describe cómo carga Traefik los plugins, y se añade una cláusula: los mismos paquetes también se compilan en una build propia de Traefik.
  2. `README.md` `## Yaegi` ("This code is interpreted inside Traefik, not compiled into it"): se reescribe. El código corre compilado y bajo Yaegi; la restricción Yaegi se mantiene porque la ruta interpretada sigue existiendo.
  3. `knowledge/devdocs/std_go_simpleredis.md` ("production is compiled into Traefik, and Yaegi is a portability and e2e-cost concern, not the speed target"): se corrige en el mismo sentido.
  La sección comparativa nueva usa la misma formulación, para que README y devdoc no se contradigan.
- **Devdocs**: una línea "Benchmark with …" en `std_go_tokenbucket.md` y `std_go_windowcounter.md` (How to use / Key files), y en `std_go_test-suites.md` que los benchmarks no corren en CI y cómo se ejecutan, incluidos los live. También se corrige la frase de `std_go_simpleredis.md` que dice que "Yaegi is a portability and e2e-cost concern, not the speed target" (respuesta 2 del solicitante).
- **Tabla del README**: se genera en esta máquina (Intel Core Ultra 7 265K, 20 hilos, Windows 11, `go1.25.6 windows/amd64`) con `-count 5` y la mediana. La tabla indica la máquina, la versión de Go y el comando para reproducirla (respuesta 1 del solicitante).

### Cambio de alcance: optimizar los dos costes que crecen con el número de claves

Respuesta 3 del solicitante (2026-11-21, a través del conductor): las dos optimizaciones entran en este cambio. Restricción: el comportamiento observable no cambia. Eso significa las mismas decisiones allow/deny, los mismos topes, la misma semántica de error y de caída de Redis, compatible con Yaegi y sin dependencias nuevas. Los benchmarks nuevos deben mostrar el antes y el después.

Es un cambio de alcance pedido por el solicitante. No es un reshape hacia una unidad existente, así que no lleva fila en `deviations.md` (`skill:opd-workflow:Deviations`: "a row with no owner is not a reshape, it is a scope change"). Prepare volvió a pasar y reescribió `requirement.md` (src `b7253c9e5ff28b3e`, `qualified-with-gaps`): ahora Desired 4 y 5 son estas optimizaciones y la corrección del devdoc. Los dos huecos que marcó prepare ("mismos topes" en (a) y "misma semántica ante errores" en (b)) se cierran con los criterios de aceptación de las secciones (a) y (b).

**Orden de implement (antes/después):** primero se hace commit de los benchmarks sin tocar el código de producción y se mide la base. Luego se optimiza y se mide otra vez con los mismos benchmarks y el mismo comando. Los dos conjuntos de números van en la tarjeta del PR, con los SHAs de los dos commits. El README publica las cifras finales y, para las dos filas optimizadas, una línea con la cifra anterior y el commit base.

#### (a) `tokenbucket.Memory`: alta de una fuente nueva

Hoy (`tokenbucket/memory.go`):
- Cada alta llama a `dropExpired`, que recorre el mapa entero.
- Con la tabla llena, `dropOne` recorre el mapa otra vez y borra una fuente viva cualquiera (el orden de iteración del mapa de Go).
- Todo ocurre bajo `m.mu`.
- Medido: unos 0.8 ms con 60k fuentes y 1.6 ms con la tabla llena (65536).

Llamadores internos enumerados: `dropExpired` 2 (`Allow`, `dropOne`), `dropOne` 1 (`Allow`). Hay 5 sitios de test que leen o escriben `limiter.buckets[...]` directamente, y uno de ellos inserta un `memEntry` a mano (`allow_wait_duration_overflow_test.go:21`).

| Opción | Qué hace | Peor caso tras el cambio | Riesgos / coste |
|---|---|---|---|
| A. Barrer solo al llegar al tope | Se quita el `dropExpired` de cada alta. Por debajo del tope, las entradas caducadas esperan; se siguen borrando de forma perezosa al consultarlas, como hoy. | Tabla llena solo de fuentes vivas: sigue en O(n) por alta (1.6 ms), igual que hoy. | Cambio mínimo, pero no arregla el peor caso medido. La memoria máxima no cambia (el tope ya la acota). |
| B. Barrido amortizado (como mucho uno por `ttl`, o K entradas por alta) | Reparte el coste del barrido. | Con la tabla llena solo de vivas sigue en O(n), salvo que se combine con C. | Para elegir qué desalojar habría que saber si queda alguna caducada, y eso pide un recorrido completo. |
| **C. Lista intrusiva ordenada por caducidad (recomendada)** | `memEntry` recibe `prev`/`next` y su clave. Cada `Allow` mueve la entrada al final (O(1)), porque `expireAt = now + ttl` con `ttl` fijo por limitador y `now` leído bajo el mutex. Así, el orden de la lista es el orden de caducidad. En un alta, se sacan por delante las caducadas; con la tabla llena y sin caducadas, se desaloja la de delante. | O(1) por alta. | 2 punteros más y la clave por entrada. Se desaloja la fuente que lleva más tiempo sin usarse en vez de una al azar. Ningún spec fija cuál (`std_go_tokenbucket_allow` solo exige caducidad perezosa) y el tope de 65536 no cambia. Si el reloj retrocede (`SetNowForTest`, salto del reloj de pared sin lectura monotónica), la cabeza puede no ser la que caduca antes; las decisiones no cambian porque la consulta sigue comprobando `expireAt`. Los tests que insertan en `buckets` a mano tienen que pasar por la lista. |
| D. `container/list` | Igual que C, pero con la lista de la librería estándar. | O(1) | `Element.Value` es `any` y obliga a un type assertion. Bajo Yaegi, los valores que pasan por `any` ya han dado problemas (`knowledge/research/ext_geoblock_pester-integration/.sources/2026-09-08-yaegi-drops-methods-on-any.md`). C lo evita con punteros concretos. |

**Criterio de aceptación de "mismo comportamiento observable" en (a)** (Desired 4a; cierra la Unknown "qué cuenta como mismos topes"):
- **Tope:** el mapa nunca pasa de 65536 fuentes (`maxMemorySources`), igual que hoy.
- **Caducadas primero:** al dar de alta una fuente con la tabla llena, se sacan las caducadas antes que cualquier viva. Solo si no queda ninguna caducada se desaloja una viva.
- **Vuelta como bucket lleno:** una fuente desalojada o caducada vuelve con `tokens = burst` y `last = now` y consume 1, exactamente como una clave nueva hoy (`std_go_tokenbucket_allow`: "A missing key or a key whose `ttl` has elapsed SHALL start as a full bucket").
- **Decisiones sin cambios:** las decisiones de admitir, esperar o denegar de una fuente presente no cambian. `consumeOne` y el cálculo de `allowed` no se tocan.
- **Víctima no fijada:** qué fuente viva se desaloja no está fijado hoy. `dropOne` borra la primera en el orden de iteración del mapa, que no es determinista, y no hay test del tope (`requirement.md`, Current). Desalojar la menos usada es una instancia válida de "una cualquiera".
- **Spec:** propose añade a `std_go_tokenbucket_allow` un requisito que fije el tope y el orden caducadas-primero. No promete LRU salvo que propose lo decida.
- **Tests que fijan hoy la semántica de `Memory` y deben seguir pasando sin cambios:**
  - `tokenbucket/limiter_test.go`: `TestMemory_IdlePastTTLStartsFull`, `TestMemory_BurstAfterIdle`, `TestMemory_RefundWhenWaitExceedsMaxDelay`, `TestMemoryAndRedis_Agree`.
  - Repros: `TestRepro_NewKeyFillsToBurstAtEpoch`, `TestRepro_StaleNowRewindsLastDoubleRefill`, `TestRepro_NaNRateAllowFailOpen`.
  - Los de `allow_maxdelay*_test.go` y `allow_wait_duration_overflow_test.go`.
  - Los Yaegi de `limiter_yaegi_test.go`.
  - Ejecutados en esta máquina el 2026-11-21 sobre el dest: `go test -short -count=1 ./tokenbucket/` → `ok`.
- **Un test a adaptar:** `allow_wait_duration_overflow_test.go` inserta un `memEntry` directamente en `buckets`. Si C añade campos de lista, ese test pasa por un helper de test sin cambiar lo que comprueba.
- **Hueco: no hay test del tope.** Implement añade uno, que pasa antes y después de la optimización:
  - tabla llena con caducadas y vivas → se sacan las caducadas;
  - tabla llena solo de vivas → `len` se queda en 65536 y la fuente nueva arranca llena.

Recomendación: **C**. Es la única que quita el O(n) del peor caso medido sin cambiar las decisiones, el tope ni la API, y usa solo punteros de Go, así que es segura bajo Yaegi. Implement debe probar que el desalojo con la tabla llena borra primero las caducadas, que una fuente desalojada vuelve con `burst` y que los tests Yaegi siguen pasando. El benchmark "clave nueva con la tabla llena" da el antes y el después.

#### (b) `windowcounter`: flush en serie, un EVAL por clave

Hoy (`windowcounter/limiter.go` `flushPendingLocked` → `flushEvalUnlocked`):
- Manda un EVAL de `flushScript` por cada clave sucia, una detrás de otra, sin `l.mu`.
- Un error en una clave deja esa clave con `flushDelta = 0` (se reintenta en el siguiente tick), guarda el primer error y no afecta a las demás.
- Medido: unos 21 µs por clave en el falso.
- Durante una caída de Redis cada EVAL agota su propio presupuesto (`CommandTimeout` 900 ms con un reintento), así que un flush con N claves puede tardar N × presupuesto.
- Llamadores enumerados: `flushPending` 3 en producción (`Sleep`, `Close`, `flushAfterTick`) más 4 en tests; `flushPendingLocked` 1; `flushEvalUnlocked` 1.

Capacidades de SimpleRedis comprobadas:
- No tiene pipeline ni MULTI/EXEC. Los verbos públicos son `Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX` y `MSetEXAt`, y cada uno es un `exec` con su propio presupuesto.
- Ya existe un precedente de EVAL multiclave seguro en Dragonfly: `msetexFallbackScript` en `simpleredis/commands_msetex.go`, con bucle numérico sobre `#KEYS` y sin `unpack`/`table.maxn`, tope de 1024 pares.
- Dragonfly exige declarar en `KEYS` todas las claves que toca un script (`ext_dragonfly_eval`).
- `std_go_simpleredis.md`: los motores en cluster necesitan todas las claves en el mismo hash slot, y el cliente no pone hash tags ni divide. El cliente tampoco sigue `MOVED`, así que hoy no hay soporte de Redis Cluster.

| Opción | Qué hace | Riesgos / coste |
|---|---|---|
| **A. EVAL por lotes (recomendada)** | Script nuevo: un bucle sobre `KEYS[i]` con `exists`, `incrby` y `expireat`-si-era-nueva, usando `ARGV[2i-1]` y `ARGV[2i]`, con lotes de hasta K claves (por ejemplo 256, siempre ≤ 1024 como `MSetEX`). Devuelve un array plano con un valor por clave. Cada clave va dentro de `redis.pcall`, y un error se devuelve como un marcador en su posición (por ejemplo `"err:<mensaje>"` con `tostring`), así cada clave conserva su resultado y su error como hoy. | Un error a mitad de un script de Redis no deshace las escrituras anteriores. Sin `pcall`, un fallo en una clave haría que Go tratara el lote entero como fallido y lo reenviara: doble conteo. El `pcall` por clave lo evita. Un fallo de red falla el lote entero: el comportamiento es el de hoy (todas las claves se reintentan) y el flush tarda menos durante una caída. Una respuesta perdida tras aplicar el lote puede aplicar K claves dos veces; hoy ese riesgo es de una clave por vez y ya se acepta para EVAL. Un motor en modo cluster o detrás de un proxy de cluster devolvería CROSSSLOT; hoy no está soportado, pero el README y los devdocs deben decirlo. El spec `std_go_windowcounter_sync-flush` dice "one EVAL of INCRBY plus EXPIREAT … with the touched key declared in KEYS": propose tiene que reescribirlo con un MODIFIED. El falso de `windowcounter` empareja `args[1] == flushScript`, así que hay que enseñarle el script nuevo. |
| B. Pipeline en SimpleRedis | Mandar N EVAL sin esperar cada respuesta. | Capacidad pública nueva de `simpleredis` (pool, reintentos y presupuesto por salto). Es mucho más superficie que este ticket y cambia otro spec (`std_go_simpleredis_resp-commands`). Rechazada. |
| C. Flush concurrente con N workers | El mismo EVAL por clave, repartido entre goroutines. | Compite con el pool (8 sockets por defecto) que usan los Take/Peek de la ruta de petición, que pueden acabar en `ErrPoolWait`. Añade goroutines al código interpretado. No reduce los comandos. Rechazada. |

**Criterio de aceptación de "misma semántica ante errores" en (b)** (Desired 4b; cierra la Unknown "qué cuenta como misma semántica ante errores cuando varias claves viajan juntas"). El resultado por clave tiene que ser el mismo que hoy da un EVAL por clave en `flushPendingLocked`:
- **Clave cuyo INCRBY se aplicó:** `redisKnown` = valor devuelto, `unread` = false, `localDelta -= delta enviado`, `flushDelta` = 0. Cualquier éxito limpia `lastOutageErr`.
- **Clave cuya escritura falló:** `flushDelta` = 0 y `localDelta` intacto. El delta pendiente se reenvía en el siguiente flush.
- **Error guardado:**
  - Al final del flush se guarda el primer error en `lastOutageErr`/`lastOutageAt`.
  - Durante un `sync_rate`, Take/Peek buffered no contactan Redis (`skipRedisContactLocked`) y admiten con el límite local y **error nil**. No devuelven el error guardado: `std_go_windowcounter_sync-flush`, "Failed buffered flush is retained", dice "Take and Peek MUST NOT return the stored error".
  - El mensaje del conductor decía que Take/Peek "return that error". El código y el spec dicen lo contrario; este criterio sigue al código y al spec.
- **Hit en vuelo:** un hit contado mientras el lote viaja se queda en `localDelta`. Un snapshot en vuelo no se envía dos veces (`flushDelta` ≠ 0 lo excluye). El EVAL no se envía con `l.mu` tomado.
- **Sin doble conteo al reintentar:** las claves que el script aplicó no se marcan como fallidas, así que no se reenvían. Por eso cada clave va dentro de `redis.pcall` y devuelve su propio resultado o marcador de error: sin `pcall`, un error a mitad del script dejaría aplicadas las claves anteriores (Redis no deshace) mientras Go reenvía el lote entero.
- **Fallo del lote entero** (red, timeout): todas las claves del lote cuentan como fallidas, igual que hoy cuando Redis no responde a ninguna.
- **Riesgo ya aceptado, ahora por lote:** una respuesta perdida después de aplicar el script puede reaplicar el lote, como hoy puede reaplicar una clave (`std_go_simpleredis.md`: "INCR/INCRBY/EVAL can double-apply after a lost reply; that is accepted").
- **Llamadores sin cambios:** `flushAfterTick`, `Sleep` y `Close` (los tres llamadores de `flushPending`) siguen con el mismo contexto y la misma semántica.
- **Tests que fijan hoy esta semántica y deben seguir pasando sin cambios:**
  - `windowcounter/limiter_test.go`: `TestBuffered_TickerFlushesWithoutSleep`, `TestBuffered_TwoClientsShareWithoutLastWriteWins`, `TestClose_StopsTickerAndKeepsRedis`, `TestWake_StartsTickerAfterSleep`, `TestTake_BufferedPendingDeltaOutage`, `TestTake_BufferedFlushThenKillKeepsLocalCap`, `TestTake_BufferedStoredOutageSkipsShareRefreshGET`, `TestTake_BufferedTwoInstancesOutage`, `TestTake_BufferedSleepStoresFlushError`, `TestPeek_BufferedEmptyFlushThenKill`, `TestPeek_BufferedDoesNotIncrement`, `TestPeek_BufferedTakeThenPeekDenies`, `TestPeek_BufferedSkipStormDoesNotGetEveryCall`.
  - `windowcounter/repro_buffered_outage_test.go`: `TestRepro_BufferedTakeHidesRedisOutage`, `TestBuffered_PeekOnlyRecoversAfterTransientOutage`, `TestBuffered_TakeOutageSeedGetsSharedCountFromFlush`.
  - `windowcounter/repro_stale_previous_test.go`: `TestRepro_BufferedStalePreviousAtBoundary`, `TestTake_BufferedUnflushedPreviousNotGet`, `TestTake_BufferedPreviousRefreshBoundedBySyncRate`.
  - `windowcounter/repro_lock_during_get_test.go` (`TestRepro_BufferedTakeHoldsLockDuringRedisGet`), `windowcounter/repro_sleep_wake_race_test.go` (`TestRepro_SleepWakeRaceHangs`), `windowcounter/yaegi_flush_stop_test.go` (`TestYaegi_BufferedShareSleepDoesNotHang`, `TestYaegi_MethodFlushStopDoesNotHang`).
  - Live: `TestLive_Limiter` y `TestYaegiLive_RedisAndDragonfly` (`bufferedTwoClients`, `bufferedPeek`).
  - Ejecutados en esta máquina el 2026-11-21 sobre el dest: `go test -short -count=1 ./windowcounter/` → `ok`.
- **Ajuste de test permitido:** el falso de `windowcounter/fake_redis_test.go` empareja `args[1] == flushScript`, así que tiene que aprender el script por lotes. Cambia el falso, no lo que comprueban los tests.
- **Huecos: no hay test que fije hoy estos casos** (grep sin resultados en `*_test.go` para el caso en vuelo). Implement los añade, contra el falso con fallo por clave inyectable:
  - (i) un fallo por clave dentro de un lote en el que otras claves sí se aplican;
  - (ii) un Take durante un flush en vuelo que conserva su hit (escenario del spec "Take during flush keeps the new local hit");
  - (iii) que, en un lote con fallo parcial, las claves aplicadas no se reenvían.
- **Compatibilidad Dragonfly del EVAL multiclave** (cierra la Unknown):
  - Hay precedente en el repo: Dragonfly no tiene `MSETEX` (`knowledge/research/ext_dragonfly_msetex/`), así que `MSetEX` cae al `msetexFallbackScript` multiclave, y `simpleredis/commands_msetex_e2e_test.go` lo ejecuta en el job `e2e-dragonfly`.
  - Es una inferencia: no se ha medido con el script de flush. Implement lo prueba en Go E2E Dragonfly (`bufferedTwoClients` y el test nuevo del lote).
- **`simpleredis/` no cambia** (cierra la Unknown "si 4b necesita algo de simpleredis"): basta con `Eval`, que ya existe.

Recomendación: **A**. Usa solo `Eval`, que ya existe, sigue el precedente multiclave de `msetexFallbackScript`, mantiene la semántica por clave gracias a `pcall`, y reduce el flush de N viajes a ⌈N/K⌉. Implement debe probar en el falso y en live (Redis y Dragonfly):
- un error en una clave no afecta a las otras del lote;
- el `local_delta` añadido mientras el EVAL está en vuelo se conserva;
- un snapshot nunca se envía dos veces;
- en una caída, todo el lote vuelve a `flushDelta = 0` y se guarda el error.

`BenchmarkFlush` (1000 claves) da el antes y el después.

### Alternativas rechazadas

- Medir contra el falso actual tal cual: el resultado mezcla un viaje NOSCRIPT que Redis real no hace con la caché caliente (medido: el doble de coste).
- Cambiar el comportamiento por defecto del falso (caché siempre activa): rompería o vaciaría los asserts de `lastEvalCommand`; es más superficie de la necesaria.
- Un falso nuevo solo para benchmarks: duplicaría el parser RESP y la lógica de script de cada paquete (One job, one owner).
- Pre-cargar el script con `SCRIPT LOAD` desde el benchmark: SimpleRedis no expone ese verbo y la API pública está fuera de alcance.
- Medir el flush de `sync_rate` > 0 dentro del Take cronometrado con un `sync_rate` muy bajo: el flush corre en otra goroutine y su coste no se reparte por Take de forma estable; se mide aparte.
- Benchmarks Yaegi live y Yaegi en paralelo: el multiplicador de interpretación ya sale contra el falso, y la red live no lo cambia. Solo añadirían tiempo y ruido.
- Ejecutar `-bench` en CI o añadir umbrales: fuera de alcance según `requirement.md`.
- (Superada por la respuesta 3 del solicitante) Dejar como follow-up el recorrido O(n) de `Memory.Allow` y el flush en serie. Ahora se optimizan aquí (ver "Cambio de alcance").
- Para `Memory`: barrer solo al tope (A), barrido amortizado (B) y `container/list` (D). Motivos en la tabla de (a).
- Para el flush: pipeline en SimpleRedis (B) y flush concurrente (C). Motivos en la tabla de (b).
- Añadir `traefik` o `go-redis` como dependencia para medir Traefik: exclusión explícita.

### Live contract

- Benchmarks, README y devdocs: ningún contrato vigente cambia. Los benchmarks son `_test.go`, CI no cambia (`std_go_ci_test-suites`) y el README no tiene spec.
- Flush por lotes (opción b-A): cambia el contrato vigente `openspec/specs/std_go_windowcounter_sync-flush/spec.md` ("one EVAL of INCRBY plus EXPIREAT … with the touched key declared in KEYS"). Propose escribe un MODIFIED. Debe conservar en el texto que el flush no se hace bajo el mutex, que se conserva el `local_delta` añadido durante el envío, que no hay doble envío y que el error queda guardado.
- Lista de caducidad en `Memory` (opción a-C): `openspec/specs/std_go_tokenbucket_allow/spec.md` no fija ni el tope ni qué fuente se desaloja; solo exige caducidad perezosa. Propose decide si añade un requisito sobre el desalojo con la tabla llena (caducadas primero, después la menos usada), según `skill:opd-speclibrarian:Live catalog`.

## Open questions

- Q: ¿En qué modalidad corren en producción los middlewares que usan este repo (interpretado por Yaegi, compilado o ambas)?
  Rank: additive asked — edita una frase del README y añade benchmarks nuevos; Desired 1 ("Cada uno en versión compilada y en versión interpretada") y Affected ("posible ajuste de `## Yaegi`")
  Decision: resolved — el solicitante lo dijo el 2026-11-21 en el chat: corren en las dos modalidades. Los benchmarks reportan ambas, y la frase de `README.md` `## Yaegi` "interpreted inside Traefik, not compiled into it" se corrige en este cambio.
  By: explore

- Q: ¿Cómo se mide `NewRedis` y `sync_rate` 0 si el Redis falso responde siempre NOSCRIPT (un viaje extra por Eval)?
  Rank: additive asked — añade un campo opt-in, test-only, a un falso que ya existe (apagado por defecto, los 48 usos de `startTestFakeRedis` no cambian); Desired 1 pide medir "contra el Redis falso en proceso"
  Decision: assumed — caché de digests opt-in en cada `testFakeRedis` (misma forma que `simpleredis` `loadedScripts`), activada solo por los benchmarks, con la métrica `redis-cmds/op`. El README dice que las cifras del falso representan un Redis con la caché de scripts caliente y que en frío cada script cuesta un viaje extra una sola vez.
  By: explore

- Q: ¿Cómo se mide `windowcounter` con `sync_rate` > 0 (coste por Take frente al flush) y con qué `sync_rate`?
  Rank: additive asked — benchmarks nuevos; Desired 1 ("con `sync_rate` 0 y con `sync_rate` > 0")
  Decision: assumed — coste por Take con el flush real en segundo plano a `sync_rate` = 20 ms (el suelo, el peor caso de tráfico), con `redis-cmds/op`, más un `BenchmarkFlush` de 1000 claves sucias contra el falso. El probe da ≈131 ns/Take con 20 ms y ≈134 ns con 1 s, así que un solo valor basta para el coste por Take.
  By: explore

- Q: ¿Qué patrones de claves y qué variantes paralelas permiten decir algo honesto sobre la escalabilidad?
  Rank: additive asked — benchmarks nuevos; Desired 2 ("escalabilidad (varias instancias de Traefik, número de claves, caída de Redis)")
  Decision: assumed — clave caliente y 10 000 claves en rotación en todos los limitadores; para `tokenbucket.Memory`, además "clave nueva con la tabla llena". `RunParallel` solo compilado. Varias instancias y caída de Redis se describen de forma cualitativa desde el código y las specs (no se benchmarkean). El README dice que el paralelo contra el falso está limitado por el falso.
  By: explore

- Q: ¿Qué forma tienen las variantes live (`TOKENBUCKET_LIVE_*`, `WINDOWCOUNTER_LIVE_*`) y cuándo se saltan?
  Rank: additive asked — benchmarks nuevos que reutilizan `lookupLiveEngineAddrs` sin cambiarla; Desired 1 ("opcionalmente live")
  Decision: assumed — un benchmark compilado por limitador y modo, con subbenchmark por motor, en `bench_e2e_test.go`. Se salta con `-short` o con las dos variables sin definir. Prefijo de clave único por ejecución. También para `windowcounter`, por simetría (la tensión del ticket se cierra así). Sin Yaegi live.
  By: explore

- Q: ¿Es representativo de Traefik el Yaegi que usan los benchmarks?
  Rank: additive asked — benchmarks nuevos; Desired 1 ("versión interpretada bajo Yaegi")
  Decision: resolved — `knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`: Traefik v3.7.11 fija `yaegi v0.16.1`, la misma versión que este `go.mod`. El intérprete del benchmark (`interp.New(GoPath)` + `stdlib.Symbols`, sin unsafe) es el mismo que usan los tests Yaegi del repo. El README cita la versión.
  By: explore

- Q: ¿Qué hechos de Traefik Proxy `RateLimit` (OSS) y de Traefik Hub `DistributedRateLimit` se pueden citar?
  Rank: additive asked — texto nuevo del README; Desired 3 ("solo con hechos documentados")
  Decision: resolved — solo lo que recogen `ext_traefik_ratelimiter_token-bucket/` y `ext_traefik_hub_distributed-rate-limit/`. Las dos páginas oficiales de Hub no coinciden en varios detalles (lista de cabeceras, `tls.caBundle` frente a `tls.ca`, `timeout`). El README se limita a lo que dice la referencia de Hub y no enumera esos detalles. Lo no documentado (caché local, sincronización, viajes por petición, Retry-After, `burst` 0) aparece como "no documentado", nunca como una comparación de rendimiento.
  By: explore

- Q: ¿Con qué máquina, versión de Go y comando se generan los números de la tabla del README?
  Rank: additive asked — tabla nueva; Desired 2 ("indicando comando, máquina y versión de Go")
  Decision: resolved — el solicitante respondió el 2026-11-21 (respuesta 1, vía el conductor): esta máquina (Intel Core Ultra 7 265K, 20 hilos, Windows 11, `go1.25.6 windows/amd64`), `-count 5` y la mediana. La tabla indica la máquina, la versión de Go y el comando para reproducirla (`go test -run '^$' -bench . -benchtime 1s -count 5`). También aclara que el loopback TCP de Windows domina las cifras contra el falso, así que sirven para comparar entre sí y no como latencia absoluta.
  By: explore

- Q: ¿Qué devdocs actualiza este cambio, y qué se hace con `std_go_simpleredis.md` ("production is compiled into Traefik, and Yaegi is … not the speed target")?
  Rank: additive asked — edición de una frase de un packet existente y líneas añadidas en otros tres; Desired 5 y Affected (`std_go_simpleredis.md`)
  Decision: resolved — el solicitante respondió que sí. Se corrige la frase de `std_go_simpleredis.md` para que diga que el código corre compilado y bajo Yaegi, y no que Yaegi "is not the speed target". La decisión de encoder que esa frase justifica sigue en el packet, ahora con las dos modalidades. Además se añade una línea de uso de benchmarks en `std_go_tokenbucket.md`, `std_go_windowcounter.md` y `std_go_test-suites.md`.
  By: explore

- Q: ¿Qué hacer con los dos costes que crecen con el número de claves (alta O(n) bajo mutex en `tokenbucket.Memory`; flush en serie en `windowcounter`)?
  Rank: bounded asked — cambia unidades existentes con llamadores enumerados (`dropExpired` 2, `dropOne` 1, `flushPendingLocked` 1, `flushEvalUnlocked` 1, `flushPending` 3 en producción y 4 en tests; raíces `tokenbucket/`, `windowcounter/`); Desired 4 de `requirement.md` (src `b7253c9e5ff28b3e`) nombra (a) y (b)
  Decision: resolved — el solicitante respondió "optimize here" y prepare reescribió `requirement.md` (Desired 4). Las dos optimizaciones entran en este cambio sin cambiar el comportamiento observable, y los benchmarks muestran el antes y el después (Desired 1). Opciones y recomendación en Decisions → "Cambio de alcance". Las filas de `issues.md` pasan a `take`.
  By: explore

- Q: ¿Con qué diseño se elimina el recorrido O(n) en las altas de `tokenbucket.Memory`?
  Rank: bounded asked — cambia `tokenbucket/memory.go` (`Allow`, `dropExpired` 2 llamadores, `dropOne` 1) y 5 sitios de test que tocan `buckets` directamente; Desired 4a
  Decision: assumed — opción C: lista intrusiva ordenada por caducidad (`key`/`prev`/`next` en `memEntry`, `head`/`tail` en `Memory`, mover al final en cada `Allow`, sacar por delante; desaparecen `dropExpired` y `dropOne`). Es O(1) por alta, mantiene el tope de 65536, saca primero las caducadas y es segura bajo Yaegi (sin `any` ni `container/list`). Con la tabla llena y sin caducadas se desaloja la fuente menos usada; el spec no lo promete. Con un reloj de test que retrocede el orden puede romperse sin cambiar decisiones ni tope (aceptado). Fijado en `openspec/changes/ratelimit-benchmarks-key-scaling/design.md` D3.
  By: propose

- Q: ¿Con qué diseño se elimina el flush en serie (un EVAL por clave) de `windowcounter`?
  Rank: bounded asked — cambia `flushPendingLocked` / `flushEvalUnlocked` (1 llamador cada uno; `flushPending` 3 en producción y 4 en tests), el `flushScript` y el contrato vigente `std_go_windowcounter_sync-flush`; Desired 4b
  Decision: assumed — opción A: un EVAL por lotes de K = 256 claves (el spec fija solo el techo de 1024, como `MSetEX`), con `redis.pcall` por clave y una posición por clave en la respuesta: entero si se aplicó, cadena `err:<mensaje>` si falló (nunca una tabla anidada, que `simpleredis.Eval` rechaza). Sin `unpack` ni `table.maxn` (Dragonfly) y con todas las claves en `KEYS`. Un script único sustituye a `flushScript` (una clave es un lote de uno). Propose no puede medir K antes de que exista el código: 256 sale del tamaño de petición (~15–20 KB) y del tiempo de script; implement solo lo cambia dentro de 64–1024 con evidencia de `BenchmarkFlush` y lo anota aquí. Se rechazan el pipeline y el flush concurrente. Restricción a documentar: un motor en modo cluster devolvería CROSSSLOT (hoy el cliente tampoco soporta cluster). Detalle en `design.md` D4.
  By: propose

- Q: El README dice "Why this exists: … It does not compile your plugin into Traefik". ¿También se cambia? (Tension "Ticket vs README vs devdoc")
  Rank: additive asked — edición de texto del README y de un devdoc; Affected ("`README.md` (… posible ajuste de la introducción y de `## Yaegi`)") y Desired 5
  Decision: resolved — la respuesta 1 del solicitante cubre las tres frases: la introducción (se conserva, con una cláusula sobre la build compilada), `## Yaegi` (se reescribe) y `std_go_simpleredis.md` (se corrige). Detalle en Decisions → "Modalidad de ejecución, tres frases a la vez". La sección nueva usa la misma formulación.
  By: explore

- Q: ¿Qué cuenta como "mismos topes" en la optimización (a) de `tokenbucket.Memory`?
  Rank: bounded asked — cambia el desalojo de `tokenbucket/memory.go` (`dropOne` 1 llamador, `dropExpired` 2) y añade un requisito a `std_go_tokenbucket_allow`; Desired 4a ("mismos topes")
  Decision: assumed — el criterio de Decisions → (a): tope de 65536, caducadas primero, la fuente desalojada vuelve como bucket llena, y las decisiones de una fuente presente no cambian. Cuál de las vivas se desaloja no está fijado hoy (`dropOne` usa el orden de iteración del mapa; no hay test), así que desalojar la menos usada cumple "una cualquiera". Propose lo fijó en el requisito ADDED "Memory store caps sources and removes expired ones first" de `std_go_tokenbucket_allow` (tope, caducadas primero, vuelta como bucket llena); no promete LRU y deja la víctima sin especificar. Implement añade el test del tope que hoy falta.
  By: propose

- Q: ¿Qué cuenta como "misma semántica ante errores" cuando el flush (b) envía varias claves en un EVAL?
  Rank: bounded asked — cambia `flushPendingLocked` / `flushEvalUnlocked` y el contrato `std_go_windowcounter_sync-flush`; Desired 4b ("misma semántica ante errores y caída de Redis")
  Decision: assumed — el resultado por clave es igual al de hoy: si se aplicó, sale de pendientes; si falló, conserva su delta y el primer error queda guardado. Take/Peek siguen admitiendo con el límite local y error nil durante un `sync_rate` (el spec prohíbe devolver el error guardado). No hay doble conteo al reintentar gracias a `redis.pcall` por clave. Los tests existentes que lo fijan están en Decisions → (b) y deben pasar sin cambios. Implement añade los tres casos que hoy no tienen test. Propose añade dos precisiones: una respuesta con un número de posiciones distinto al de claves falla todas las claves del lote (`redis:issue?`), y un fallo de `EXPIREAT` después de un `INCRBY` aplicado no se comprueba, porque marcar la clave como fallida provocaría doble conteo al reintentar (hoy ese caso, que en la práctica no ocurre, falla el EVAL entero y doble cuenta). Spec: MODIFIED de "Buffered mode shares one limit without last-write-wins" y "Flush EVAL integer parse keeps the cause", ADDED "Buffered flush batches keys with a per-key outcome" en `std_go_windowcounter_sync-flush`.
  By: propose

- Q: ¿Funciona en Dragonfly un flush que toca varias claves en un EVAL?
  Rank: additive asked — script nuevo que solo usa claves declaradas en `KEYS`; Desired 4b ("seguras bajo Yaegi, sin dependencias nuevas") y la Unknown de Dragonfly
  Decision: resolved — sí. `windowcounter/limiter_flush_batch_e2e_test.go` (`TestLive_FlushBatchSeveralKeys`: 266 claves, dos EVAL, el primero con 256) pasó en CI `Go E2E Redis` y `Go E2E Dragonfly` sobre `049e9ab9` (run 44890987232); esos jobs fijan `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY` y corren sin `-short` (`.github/workflows/ci.yml`); no se leyó el log del job.
  By: implement

- Q: ¿Necesita la optimización (b) algo de `simpleredis/` que hoy no expone?
  Rank: additive asked — ningún cambio en `simpleredis/`; Out of scope ("cambiar `simpleredis/` salvo que el punto 4b lo exija")
  Decision: resolved — no. Basta con `Eval`, que ya existe. Se descartan el pipeline y MULTI/EXEC, que SimpleRedis no tiene (verbos públicos revisados en `simpleredis/commands*.go`).
  By: explore

- Q: ¿Cómo presenta la tabla del README el "antes" de las dos optimizaciones?
  Rank: additive asked — tabla nueva; Desired 1 ("mostrar el antes y el después") y Desired 2
  Decision: assumed — implement hace primero un commit solo con los benchmarks, mide la base y luego optimiza y mide otra vez con el mismo comando. La tarjeta del PR lleva las dos series con los SHAs de los dos commits. El README publica la serie final y, en las filas de "clave nueva con la tabla llena" y `BenchmarkFlush`, una columna "antes (commit `<sha>`)", para que se pueda reproducir con `git checkout`. Implement: base en `d940bf43` (`bench-before.txt`); la primera serie "después" en `ae7f4a9a` mostró que la ruta Yaegi de `Memory` con clave caliente subía de ≈8,2 µs a ≈12,3 µs por mover siempre la entrada en la lista, así que `a51d20a3` deja en su sitio la entrada que ya es la cola; la serie "después" publicada es la de `a51d20a3` (`bench-after.txt`; la de `ae7f4a9a` queda en `bench-after-ae7f4a9a.txt`).
  By: implement

- Q: ¿Qué hacer con la diferencia entre go1.25.6 (máquina de la tabla) y `go 1.21` (`go.mod`)?
  Rank: additive asked — texto de la tabla; Desired 2 ("máquina de referencia … go1.25.6") y la Tension "Máquina de referencia vs módulo"
  Decision: resolved — el solicitante fijó go1.25.6 (respuesta 1). La tabla indica que los números son de go1.25.6 y que el suelo declarado del módulo es `go 1.21`, sin decir nada sobre el rendimiento con 1.21.
  By: explore
