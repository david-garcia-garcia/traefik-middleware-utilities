# Explore

## Concepts

### Unidades que toca este cambio

| Unidad | Ruta | Papel en este cambio |
|---|---|---|
| Token bucket en memoria | `tokenbucket/memory.go` (`Memory.Allow`) | Se mide: mapa en proceso bajo un `sync.Mutex`, tope `maxMemorySources` = 65536 |
| Token bucket Redis | `tokenbucket/redis.go` (`Redis.Allow`) | Se mide: un `simpleredis.Eval` (EVALSHA y, si responde NOSCRIPT, EVAL) del `allowScript` por llamada |
| Window counter | `windowcounter/limiter.go` (`Take`: `takeExact` / `takeBuffered`, `flushPendingLocked`) | Se mide: `sync_rate` 0 = EVAL + GET por Take; `sync_rate` > 0 = memoria con GET ocasional y flush en un `time.AfterFunc` |
| Eval de SimpleRedis | `simpleredis/commands_eval.go` (`Eval`) | No cambia. EVALSHA primero; con NOSCRIPT reenvía EVAL. No guarda estado de "script cargado" |
| Redis falso de tokenbucket | `tokenbucket/fake_redis_test.go` (`testFakeRedis`, `startTestFakeRedis(t *testing.T)`) | Test-only. Responde siempre NOSCRIPT a EVALSHA |
| Redis falso de windowcounter | `windowcounter/fake_redis_test.go` (ídem) | Test-only. Responde siempre NOSCRIPT a EVALSHA |
| Redis falso de simpleredis (referencia) | `simpleredis/fake_redis_test.go` (`loadedScripts`) | Precedente en el repo: el falso guarda el digest en cada EVAL y responde a EVALSHA si lo conoce |
| Arnés Yaegi | `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go` (`writeGopath*`, `copyNonTestGo`, `callerDir`, `writeGopathFile`, todos con `t *testing.T`) | Se reutiliza en los benchmarks interpretados |
| Patrón de benchmark Yaegi | `simpleredis/interpretedcost_test.go` `BenchmarkYaegiGet` | Patrón a copiar: GOPATH temporal, paquete probe con `Loop(count)`, un `Eval` de calentamiento y luego un `Eval(b.N)` cronometrado |
| Selección de motor live | `tokenbucket/limiter_e2e_test.go`, `windowcounter/limiter_e2e_test.go` (`lookupLiveEngineAddrs` es pura; `liveEngineAddrs` / `runForEachLiveEngine` usan `*testing.T`) | Se reutiliza `lookupLiveEngineAddrs` para los benchmarks live |
| README | `README.md` `## Window counter`, `## Token bucket`, `## Yaegi`, `## Tests` | Nueva sección comparativa con tabla; se corrige la frase de `## Yaegi` |
| Devdocs | `knowledge/devdocs/std_go_tokenbucket.md`, `std_go_windowcounter.md`, `std_go_test-suites.md` | Uso: cómo reproducir los benchmarks |

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
- **README `## Yaegi`**: se corrige "interpreted inside Traefik, not compiled into it" para decir que el código corre en las dos modalidades (decisión del solicitante). La restricción Yaegi se mantiene, porque la ruta interpretada sigue existiendo.
- **Devdocs**: una línea "Benchmark with …" en `std_go_tokenbucket.md` y `std_go_windowcounter.md` (How to use / Key files), y en `std_go_test-suites.md` que los benchmarks no corren en CI y cómo se ejecutan, incluidos los live.

### Alternativas rechazadas

- Medir contra el falso actual tal cual: el resultado mezcla un viaje NOSCRIPT que Redis real no hace con la caché caliente (medido: el doble de coste).
- Cambiar el comportamiento por defecto del falso (caché siempre activa): rompería o vaciaría los asserts de `lastEvalCommand`; es más superficie de la necesaria.
- Un falso nuevo solo para benchmarks: duplicaría el parser RESP y la lógica de script de cada paquete (One job, one owner).
- Pre-cargar el script con `SCRIPT LOAD` desde el benchmark: SimpleRedis no expone ese verbo y la API pública está fuera de alcance.
- Medir el flush de `sync_rate` > 0 dentro del Take cronometrado con un `sync_rate` muy bajo: el flush corre en otra goroutine y su coste no se reparte por Take de forma estable; se mide aparte.
- Benchmarks Yaegi live y Yaegi en paralelo: el multiplicador de interpretación ya sale contra el falso, y la red live no lo cambia. Solo añadirían tiempo y ruido.
- Ejecutar `-bench` en CI o añadir umbrales: fuera de alcance según `requirement.md`.
- Optimizar `Memory.Allow` (recorrido O(n) en altas) o el flush en serie: fuera de alcance ("optimizar los limitadores a raíz de los resultados"). Quedan como follow-up en `issues.md`.
- Añadir `traefik` o `go-redis` como dependencia para medir Traefik: exclusión explícita.

### Live contract

`no live contract`: los benchmarks son `_test.go` y no cambian ningún comportamiento especificado en `openspec/specs/std_go_tokenbucket_*`, `std_go_windowcounter_*` ni `std_go_ci_test-suites` (CI no cambia). El README no tiene spec. Propose confirma con `skill:opd-speclibrarian:Live catalog`.

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
  Decision: assumed — esta máquina de desarrollo (Intel Core Ultra 7 265K, 20 hilos, Windows 11, `go1.25.6 windows/amd64`, distinto del suelo `go 1.21` del módulo), con `go test -run '^$' -bench . -benchtime 1s -count 5` y la mediana. La tabla aclara que el loopback TCP de Windows domina las cifras contra el falso: sirven para comparar entre sí, no como latencia absoluta. Si el solicitante prefiere Linux o la versión de CI (1.21), basta con volver a ejecutar.
  By: explore

- Q: ¿Qué devdocs actualiza este cambio, y qué se hace con `std_go_simpleredis.md` ("production is compiled into Traefik, and Yaegi is … not the speed target")?
  Rank: additive incidental — ningún criterio nombra `std_go_simpleredis.md`; las líneas en los packets de los limitadores son añadidos
  Decision: assumed — se añade una línea de uso de benchmarks en `std_go_tokenbucket.md`, `std_go_windowcounter.md` y `std_go_test-suites.md`. `std_go_simpleredis.md` no se edita en este cambio: esa frase justifica una decisión de encoder de SimpleRedis, no la de los limitadores. Devdocs impact la revisa a la luz de la respuesta 1.
  By: explore

- Q: ¿Qué hacer con los dos costes que crecen con el número de claves (alta O(n) bajo mutex en `tokenbucket.Memory`; flush en serie en `windowcounter`)?
  Rank: additive incidental — solo documentación del comportamiento actual; arreglarlo está en Out of scope ("optimizar los limitadores a raíz de los resultados")
  Decision: assumed — el README los documenta con números de los benchmarks y no se cambia código. Follow-ups en `issues.md` con sus ficheros `knowledge/debt/`.
  By: explore

- Q: El README dice "Why this exists: … It does not compile your plugin into Traefik". ¿También se cambia?
  Rank: additive asked — edición de texto del README; Affected ("`README.md` (… posible ajuste de `## Yaegi`)") y respuesta 1 del solicitante
  Decision: assumed — esa frase describe cómo carga Traefik los plugins, y eso sigue siendo cierto. Se conserva y se añade una cláusula: los mismos paquetes también se compilan en una build propia de Traefik. El cambio fuerte es la frase de `## Yaegi`.
  By: explore
