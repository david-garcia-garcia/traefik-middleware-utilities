# Explore

## Concepts

Este ticket no cambia producto. Fija con tests el comportamiento que dest ya tiene: Memory no retiene buckets caducados cuando entra una fuente nueva, y Redis/Dragonfly ponen `EXPIRE` entero en cada EVAL.

Unidades que el cambio tocaría:

- `Memory` — `tokenbucket/memory.go`. Un bucket por fuente opaca, tope 65536, lista de caducidad `head`/`tail`.
- `makeRoomLocked` — mismo fichero. Antes de guardar una fuente ausente o caducada, borra la cabeza mientras esté caducada; si el mapa sigue en el tope, borra la cabeza aunque viva.
- `allowScript` — `tokenbucket/lua.go`. Tras `HSET last,tokens` llama `redis.call('expire', key, ttl)`.
- `Redis.Allow` — `tokenbucket/redis.go`. Pasa `ttlSeconds` como ARGV[3]. Firma dest: `(Status, error)`.
- Gating live — `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`). Skip con `-short` o sin `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`.
- Specs vivos — `openspec/specs/std_go_tokenbucket_allow/spec.md` (tope y caducadas primero) y `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` (`EXPIRE` en segundos enteros).
- Tests de dest a conservar — `tokenbucket/memory_test.go` (tope 65536) y `TestLive_Limiter` (no lee `PTTL`).

Conexión:

```
Allow(ctx, key)  [caller owns the opaque key]
        │
        ├─ Memory ──────────────────────────────────────────────┐
        │   lock → lookup                                       │
        │     expired this key → removeLocked                   │
        │     missing → makeRoomLocked (expiry-list head)       │
        │             → store full bucket → consumeOne          │
        │                                                       │
        └─ Redis ─ Eval allowScript ─ HSET ─ EXPIRE ttlSeconds ─┘
                                              │
                                              ▼
                                    live PTTL in (0, ttl]
```

Call sites que importan (acotados, no hay reshape de producto):

- `makeRoomLocked`: 1 sitio (`Memory.Allow` cuando la entrada falta). Raíces: `tokenbucket/` y grep de repo `dropExpired`/`dropOne` (solo quedan en `backendbackoff/allow.go`, fuera de alcance).
- `runForEachLiveEngine` en `tokenbucket/`: definición más 2 llamadas dest (`TestLive_Limiter`, `TestYaegiLive_RedisAndDragonfly`). El e2e nuevo reutiliza esa función; no la de `windowcounter/` ni `simpleredis/` (firmas distintas).
- `client.Eval` + `ScriptSHA1Hex` para leer `PTTL`: patrón in-tree en `windowcounter/limiter_e2e_test.go` (`liveTTLScript`). `simpleredis` no exporta `Pttl`.

Reproduce: **not reproduced** (el fallo es de Traefik `ttlmap`; aquí el camino correcto ya corre). Probe desechable (borrado; no se commitea): `go test -v -count=1 -timeout 60s -run TestExplore13704_ ./tokenbucket/` con `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` y `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380`.

- Memory, diez fuentes ociosas más `ttl+9s` más `"fresh"`: `len(buckets) == 1`. PASS.
- Redis live tras tres `Allow`, `ttl` 7s: `PTTL after Allow: 6999 ms`. PASS.
- Dragonfly live, misma aserción: `PTTL after Allow: 6999 ms`. PASS.

En dest no existen `tokenbucket/traefik_issue13704_test.go` ni `tokenbucket/traefik_issue13704_e2e_test.go`. Los tests de referencia viven solo en el árbol de repros (lectura): `TestTraefik13704_MemoryReclaimsExpiredBuckets` y `TestTraefik13704_RedisBucketExpires`. Esos cuerpos llaman `Allow` con tres retornos (API anterior).

Identidad: este trabajo no reconstruye dirección, usuario, tenant ni Host. `Allow` recibe una clave opaca; el spec vivo ya dice que el caller la posee. Los tests usarán cadenas sintéticas.

Outside facts: in-tree `knowledge/research/ext_redis_expire/` (`EXPIRE` en segundos enteros). `PTTL` no tiene ficha propia; se midió en los dos motores locales (6999 ms con `ttl` 7s). No se escribe research nuevo. El issue upstream https://github.com/traefik/traefik/issues/13704 es el origen del defecto (Traefik); no hay PR upstream.

Follow-up `Memory.dropExpired` / `dropOne`: dest ya no tiene esas funciones. Las sustituyó `makeRoomLocked` (lista de caducidad) en `2026-11-21-ratelimit-benchmarks-key-scaling`. Ya hay `BenchmarkMemoryAllow` y `BenchmarkYaegiMemoryAllow`. `devstate/issues.md` marca esa fila `[x]` Taken. No se abre `knowledge/debt/` que nombre APIs ausentes. `backendbackoff.Gate` sigue con `dropExpired`/`dropOne` (deuda existente `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`, Out of scope).

## Decisions

- Seam: solo tests de regresión autocontenidos. `tokenbucket/traefik_issue13704_test.go` (Memory, diez+uno = un bucket) y `tokenbucket/traefik_issue13704_e2e_test.go` (live `PTTL` en `(0, ttl]`). Sin cambio en `memory.go`, `lua.go`, `redis.go`.
- Adaptar la referencia a `(Status, error)`. Helpers locales con sufijo `13704` en el propio fichero e2e (`liveKey13704`, `evalKey13704`). Reutilizar `runForEachLiveEngine` / `waitLiveClient` de `limiter_e2e_test.go`. Reloj de test congelado (`SetNowForTest`) para Memory, como la referencia y como `memory_test.go`.
- Contrato vivo: fold de un escenario en `std_go_tokenbucket_allow` para el caso diez ociosas + una nueva = una fuente almacenada (el SHALL de caducadas-primero ya existe; los escenarios actuales solo muestran el tope 65536). No añadir un SHALL de `PTTL`; `std_go_tokenbucket_lua-eval` ya exige `EXPIRE` entero. El e2e observa el TTL restante del motor.
- Follow-up de bancos `dropExpired`/`dropOne` en Memory: no tomado y no reabierto. Dest ya tiene lista O(1) y bancos. Explore no implementa un banco nuevo.

Rechazado:

- Cambiar producto en Memory/Redis para “arreglar” #13704 — el fallo no se da; Out of scope.
- Reintroducir `dropExpired`/`dropOne` o medirlos aquí — APIs ausentes; bancos ya existen; Design dice no tomado.
- Copiar `traefik_issues_test.go` / `traefik_issues_e2e_test.go` del árbol de repros — tres retornos, helpers compartidos (`liveKey`, `evalKey`, `countAdmits`) que chocarían con PRs paralelos.
- Fichero helper compartido entre issues, o `*_yaegi_test.go` extra para #13704 — el brief nombra dos ficheros; Yaegi de tope/caducidad ya está en `TestYaegi_MemoryCapAndExpiry`.
- Rediseñar el brief — Design vinculante.

Live contract: `openspec/specs/std_go_tokenbucket_allow/spec.md`, `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`. Cambios OpenSpec ajenos en el worktree (`reclaim-alias`) no se tocan.

## Open questions

- Q: Cómo adaptar los tests de referencia a `Allow(ctx, key) (Status, error)` sin copiar helpers de otros issues?
  Rank: additive asked — ficheros de test que este cambio crea; la línea Tests del Design nombra ficheros autocontenidos con sufijo 13704
  Decision: assumed — llamar `Allow` como `(status, err)` y fallar si `err != nil`. El test Memory no usa `countAdmits`; cuenta `len(limiter.buckets)` bajo el mutex. El e2e declara `liveKey13704` y `evalKey13704` en `traefik_issue13704_e2e_test.go` (Eval `pttl` con `ScriptSHA1Hex`, KEYS[1] para Dragonfly). No copiar `liveKey`/`evalKey` del árbol de repros ni de otros issues.
  By: explore

- Q: Propose añade un escenario al spec vivo para diez+uno / PTTL o deja el catálogo como está?
  Rank: additive incidental — escenario nuevo en un leaf que ya existe; el Design dice "No product change" y no nombra un delta de spec
  Decision: assumed — fold de un escenario en el requisito "Memory store caps sources and removes expired ones first" de `std_go_tokenbucket_allow`: diez fuentes ociosas más allá de `ttl` más una nueva dejan exactamente una fuente almacenada. No añadir un SHALL de `PTTL` en `std_go_tokenbucket_lua-eval`; el `EXPIRE` entero ya es el contrato; el e2e observa el TTL restante.
  By: explore

- Q: Comportamiento de `PTTL` en Dragonfly frente a Redis en los motores locales?
  Rank: additive asked — el e2e de este cambio afirma `(0, ttl]` en ambos motores; Desired 2 nombra Redis y Dragonfly
  Decision: resolved — la misma aserción en ambos vía `runForEachLiveEngine`. Medido en este explore: Redis 6999 ms y Dragonfly 6999 ms con `ttl` 7s (ambos en `(0, 7000]`). `ext_redis_expire` cubre `EXPIRE` entero; Dragonfly EVAL no documenta incompatibilidad de `PTTL`. Implement corre el e2e contra `TOKENBUCKET_LIVE_*`; un `-1` o `0` es fallo de test, no cambio de producto.
  By: explore

- Q: El follow-up de `Memory.dropExpired` (scan O(n) bajo mutex) y `dropOne` (víctima arbitraria) sigue siendo una nota a escribir en `knowledge/debt/`?
  Rank: additive incidental — Desired 4 y Out of scope dicen no tomado; dest ya sustituyó esas APIs
  Decision: resolved — no escribir deuda que nombre `dropExpired`/`dropOne` en Memory. La fila de `issues.md` ya está `[x]` Taken (`makeRoomLocked` + `BenchmarkMemoryAllow` / `BenchmarkYaegiMemoryAllow`). Explore no implementa el banco. `backendbackoff.Gate` queda en la deuda existente, fuera de alcance.
  By: explore

- Q: El test Memory usa reloj de test o espera el `ttl` real, como podría leerse el brief?
  Rank: additive asked — Tests nombra `TestTraefik13704_MemoryReclaimsExpiredBuckets`, que congela el reloj
  Decision: assumed — `SetNowForTest` y salto `ttl+9s`, igual que la referencia y que `memory_test.go`. No dormir el `ttl` de pared.
  By: explore

- Q: Hace falta un `traefik_issue13704_yaegi_test.go` además de los dos ficheros nombrados?
  Rank: additive incidental — Tests nombra dos ficheros; no un fichero Yaegi
  Decision: assumed — solo esos dos. El tope/caducidad interpretado ya lo cubre `TestYaegi_MemoryCapAndExpiry`.
  By: explore
