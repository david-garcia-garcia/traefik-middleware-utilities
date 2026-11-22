# Explore

## Concepts

Unidades que este cambio toca (tests de regresión; sin cambio de producto):

- `Memory.Allow` — `tokenbucket/memory.go` — consume in-process. Toma `m.mu` (`Lock` en `memory.go:71`, `Unlock` diferido) y lo mantiene en la búsqueda en `buckets`, el borrado por TTL, `makeRoomLocked`/`storeLocked` si la clave falta, `consumeOne` y la escritura de `expireAt`. No hay ventana Get/Set.
- `Redis.Allow` — `tokenbucket/redis.go` — un `Eval` de `allowScript` (`redis.go:67`). Go no hace GET/SET del hash.
- `allowScript` — `tokenbucket/lua.go` — dueño atómico de `HGETALL`/`HSET`/`EXPIRE`.
- `Status.Allowed` — `tokenbucket/status.go` — verdadero cuando la espera en microsegundos es ≤ `maxDelay` truncado. Con 1 req/min, burst 5 y maxDelay 500 ms, las admite más allá de burst tienen espera de ~60 s y `Allowed` es falso.
- `runForEachLiveEngine` / `waitLiveClient` — `tokenbucket/limiter_e2e_test.go` — puerta skip/`-short`/LIVE y espera con `Config{Host: addr}` (PoolSize por defecto 8).
- Tests nuevos — `tokenbucket/traefik_issue14328_test.go` y `tokenbucket/traefik_issue14328_e2e_test.go` (aún no existen en dest).
- Contrato vivo — `openspec/specs/std_go_tokenbucket_allow/spec.md`, `std_go_tokenbucket_lua-eval/spec.md`, `std_go_tokenbucket_status/spec.md`. No hay escenario de 16 primeras peticiones simultáneas contra burst.

```
16 goroutines --barrier--> Allow(ctx, opaqueKey)
                               |
              +----------------+----------------+
              v                                 v
        Memory.Allow                      Redis.Allow
        m.mu across lookup/               one Eval(allowScript)
        create/consume                    HGETALL + HSET + EXPIRE
              |                                 |
              +--------> Status.Allowed (wait us <= maxDelay us)
```

Call sites que importan (raíces: `tokenbucket/**`, repo `waitLiveClient(` y `runForEachLiveEngine(`):

- `waitLiveClient` en `tokenbucket`: 1 definición (`limiter_e2e_test.go:210`) y 4 llamadas (`limiter_e2e_test.go:92,119,120`; `bench_e2e_test.go:17`). `windowcounter` tiene un helper homónimo aparte (no se toca).
- `runForEachLiveEngine` en `tokenbucket`: 1 definición (`limiter_e2e_test.go:74`) y 2 llamadas (`limiter_e2e_test.go:86` `TestLive_Limiter`; `limiter_yaegi_e2e_test.go:10`).
- `Memory.Allow` / `Redis.Allow` de producto: no se cambian. Dest firma `(Status, error)`.
- Tests concurrentes en dest `tokenbucket/*_test.go`: 0 (`WaitGroup` / `barrier` / `concurrent` — no hallado).

Reproduce: **not reproduced**.

- Dest `Memory.Allow` sostiene un mutex en lookup/create/consume (`memory.go:71-95`). Dest `Redis.Allow` ejecuta un script atómico (`redis.go:67`, `lua.go` `allowScript`).
- Probe throwaway contra dest (temp module, `replace` al worktree, `Status.Allowed`): Memory 20 rondas × 16 concurrentes, max admits 5; Redis `127.0.0.1:16379` 10 rondas PoolSize 16, max admits 5; Dragonfly `127.0.0.1:16380` 10 rondas PoolSize 16, max admits 5. Ninguna ronda superó burst.
- Tests de referencia (solo lectura, worktree `…-tokenbucket-traefik-upstream-repros`): `TestTraefik14328_MemoryConcurrentFirstRequests` PASS (0.00s, `-short`); `TestTraefik14328_RedisConcurrentFirstRequests` PASS redis+dragonfly (0.02s, LIVE `16379`/`16380`).
- El análisis C1 (`TRAEFIK-UPSTREAM-ISSUES.md`) coincide; las líneas citadas allí (`memory.go:59`) se desplazaron — el mutex está en `memory.go:71`. El comportamiento no.

Hechos externos:

- `knowledge/research/ext_traefik_ratelimiter_token-bucket/` — ttlmap in-memory vs Lua Redis atómico.
- https://github.com/traefik/traefik/issues/14328 (confirmado: Get/Set del ttlmap con locks separados).
- https://github.com/traefik/traefik/issues/13957 (primera mención; TTL es otro ticket).
- C1 en `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md` (solo lectura).

## Decisions

- Sin cambio de producto. No tocar `tokenbucket/memory.go`, `redis.go` ni `lua.go`.
- Añadir `tokenbucket/traefik_issue14328_test.go` (`TestTraefik14328_MemoryConcurrentFirstRequests`: 20 rondas) y `tokenbucket/traefik_issue14328_e2e_test.go` (`TestTraefik14328_RedisConcurrentFirstRequests`: 10 rondas, Redis y Dragonfly vivos).
- Carga: 16 goroutines, barrera compartida (`ready` WaitGroup + `close(start)`), 1 req/min, burst 5, maxDelay 500 ms, ttl 120 s. Contar `Status.Allowed`. Afirmar admits ≤ burst. Fallo de `Allow` falla el test.
- Helpers solo en esos archivos, sufijo `14328` (`concurrentFirstAdmits14328`, `liveKey14328`, `waitLiveClient14328`). No extraer a `limiter_e2e_test.go`. No copiar el archivo de referencia (mezcla otros issues; firma Allow de tres valores).
- Puerta viva: `runForEachLiveEngine` + `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY` (igual que `TestLive_Limiter`). Cliente con `PoolSize: 16`, `MaxIdleConns: 16`, `PoolTimeout: 2s` — no `waitLiveClient` (PoolSize 8). Desvío registrado.
- Compilar contra dest `(Status, error)`. El repro de referencia desempaqueta tres valores; no copiar esa llamada.
- Propose añade un escenario al contrato vivo (`std_go_tokenbucket_allow` y `std_go_tokenbucket_lua-eval`); no hay carpeta spec nueva.
- El cuerpo del PR dice que el defecto upstream **no ocurre aquí** y por qué (mutex único en Memory; script atómico en Redis) y nombra https://github.com/traefik/traefik/issues/14328 y https://github.com/traefik/traefik/issues/13957. No hay PR upstream de #14328.
- Si un run posterior viera admits > burst: parar. No inventar arreglo de producto.

Rechazado:

- Arreglar Traefik o parchear `Memory`/`Redis`/`allowScript` — Out of scope; el defecto no se da aquí.
- Añadir `PoolSize` a `waitLiveClient` — helper compartido, 4 llamadas en `tokenbucket`; Out of scope de helpers compartidos.
- Copiar `traefik_issues_test.go` entero — mezcla #13957/#13704/… y firma Allow antigua.
- Caso de control TTL en este par — el repro 14328 no lo tiene; eso es #13957.
- Medir `dropExpired` / lista de caducidad (C2) — Out of scope.
- Fichero Yaegi de este issue — el requirement no lo pide.

Contrato vivo: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`, `std_go_tokenbucket_status`.

## Open questions

- Q: ¿El par de referencia 14328 incluye un caso de control aparte de «admits ≤ burst»?
  Rank: additive asked — tests nuevos que este cambio crea; Desired: «Conservar los casos de control si el repro de referencia los tiene»
  Decision: resolved — Memory y Redis de referencia no tienen subtest de control tipo TTL; solo admits ≤ burst. El test nuevo afirma lo mismo.
  By: explore

- Q: ¿Cómo obtener PoolSize 16 sin cambiar `waitLiveClient` (PoolSize por defecto 8)?
  Rank: additive asked — test e2e nuevo que este cambio crea; Desired nombra PoolSize 16 y la puerta viva `runForEachLiveEngine` / `waitLiveClient`
  Decision: assumed — `runForEachLiveEngine` para skip/tabla de motores; cliente en el archivo nuevo con PoolSize 16, MaxIdleConns 16, PoolTimeout 2s (como el repro). Espera de readiness en `waitLiveClient14328` local. No cambiar `waitLiveClient` (4 llamadas en `tokenbucket/`, buscado `waitLiveClient(` en el repo).
  By: explore

- Q: ¿Se reproduce el defecto en dest (admits > burst)?
  Rank: additive asked — Unknowns pide los números en este worktree; Desired es TEST ONLY
  Decision: resolved — not reproduced. Probe dest: Memory 20×16 max admits 5; Redis 16379 10×16 max 5; Dragonfly 16380 10×16 max 5. Referencia PASS. Mutex único y script atómico en dest.
  By: explore

- Q: ¿Qué firma de `Allow` usan los tests nuevos?
  Rank: additive asked — Desired: «Compilación contra el Allow de dest: (Status, error); contar Status.Allowed»
  Decision: resolved — dest es `(Status, error)` (`memory.go:67`, `redis.go:54`). Contar `status.Allowed`. No desempaquetar tres valores.
  By: explore

- Q: ¿Qué hacer si un run posterior admitiera más de burst?
  Rank: additive asked — Out of scope: no cambiar `memory.go` / `redis.go` / `lua.go`; Desired TEST ONLY
  Decision: assumed — parar (`blocked`); no inventar arreglo de producto. Hoy dest no supera burst.
  By: explore

- Q: ¿Hay que añadir un escenario al spec vivo de 16 primeras peticiones concurrentes?
  Rank: additive incidental — contrato existente `std_go_tokenbucket_allow` / `lua-eval` sin ese escenario; Affected no lista specs; es medio para alinear el contrato con los tests
  Decision: resolved — deltas ADDED en `std_go_tokenbucket_allow` (mutex, admits ≤ burst) y `std_go_tokenbucket_lua-eval` (Eval atómico, misma carga). Sin carpeta spec nueva.
  By: propose

- Q: ¿TTL 120 s (repro) o el TTL de Traefik para 1 req/min (~61 s)?
  Rank: additive asked — tests nuevos; Desired copia la carga del repro (1 req/min, burst 5, maxDelay 500 ms)
  Decision: assumed — ttl 120 s como el repro, para no mezclar caducidad #13957 en este ticket.
  By: explore

- Q: ¿Hace falta un test Yaegi de este issue?
  Rank: additive incidental — `std_go_test-suites` documenta Yaegi para escenarios Allow; el requirement no nombra un `*_yaegi*` de 14328
  Decision: assumed — no. Compiled unit + e2e vivo. Bound the ask.
  By: explore

- Q: ¿Quién es dueño de la identidad de la fuente en `Allow`?
  Rank: additive asked — spec `std_go_tokenbucket_allow` escenario «Caller owns the key»; los tests pasan una clave opaca
  Decision: resolved — el llamador. La librería no reconstruye dirección de cliente, usuario, tenant, Host ni trust hop. Los tests pasan un string (`192.0.2.1` en Memory; `liveKey14328` en Redis). Reutilizar esa clave; no derivarla de HTTP.
  By: explore

- Q: ¿Además de PoolSize 16, hay que fijar MaxIdleConns y PoolTimeout?
  Rank: additive asked — Desired nombra PoolSize 16; el repro también pone MaxIdleConns = concurrent y PoolTimeout 2s
  Decision: assumed — igual que el repro: PoolSize 16, MaxIdleConns 16, PoolTimeout 2s. Idle trim por defecto 8 recortaría el pool; PoolTimeout 200 ms por defecto puede convertir solapes en `redis:unreachable`.
  By: explore

- Q: ¿Nombres de archivo `traefik_issue14328_*.go` o el patrón `{domain}_test.go`?
  Rank: additive asked — Desired nombra `tokenbucket/traefik_issue14328_test.go` y `traefik_issue14328_e2e_test.go`
  Decision: resolved — esos nombres. `_e2e_test.go` sigue la convención de suite viva. Archivos autónomos de este issue.
  By: explore

- Q: ¿Dónde viven los helpers de barrera y clave viva?
  Rank: additive asked — Desired: «Helpers en el propio archivo con sufijo 14328»; «No crear helpers compartidos»
  Decision: resolved — `concurrentFirstAdmits14328` / `countAdmits14328` (si hace falta) / `liveKey14328` / `waitLiveClient14328` en los archivos del issue. `t.Helper()` (thelper).
  By: explore
