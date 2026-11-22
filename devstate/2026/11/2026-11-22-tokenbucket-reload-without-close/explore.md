# Explore

## Concepts

Unidades que toca este cambio (solo prueba; el producto no se mueve):

| Unidad | Ruta | Trabajo |
|---|---|---|
| Cliente RESP | `simpleredis/simpleredis.go` (`New`, `Close`) | `New` copia `Config`, arma el semáforo in-use y no marca goroutines. `Close` drena ociosos. Este ticket no lo cambia. |
| Pool TCP | `simpleredis/pool.go` (`takeIdleConn`, `release`, `parkIdleConn`) | Tras un comando limpio el socket vuelve a idle. Sin `Close` y sin más borrows, el cliente alcanzable puede conservar ociosos más allá de `IdleTimeout`. |
| Limitador Redis | `tokenbucket/redis.go` (`NewRedis`, `Allow`) | Guarda `*simpleredis.SimpleRedis`. No hay `Close` en el limitador. Dest: `Allow(ctx, key) (Status, error)`. |
| Gate live | `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`, `TestLive_Limiter`) | Skip con `-short` o sin `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`. |
| Test nuevo | `tokenbucket/traefik_issue13618_e2e_test.go` (aún no existe en dest) | Autocontenido para #13618. Proxy que cuenta sockets + helpers con sufijo `13618`. |

`reclaim/` no entra: el brief lo deja fuera.

```
recarga ×50
  simpleredis.New ──► SimpleRedis (sin goroutine de fondo)
         │
         ▼
  NewRedis(client) ──► Redis (sin Close)
         │
         ▼
  Allow ──► Eval por TCP vía countingProxy ──► Redis/Dragonfly
         │
  se descartan refs (sin Close)
         │
  GC ──► netFD stdlib cierra el fd ──► proxy.open == 0
```

Call sites que importan (raíces: dest `tokenbucket/*.go`, `simpleredis/*.go` excluyendo `*_test.go` para producción):

- `runForEachLiveEngine`: 2 llamadas en dest (`limiter_e2e_test.go` `TestLive_Limiter`, `limiter_yaegi_e2e_test.go` `TestYaegiLive_RedisAndDragonfly`). Definición en `limiter_e2e_test.go`.
- `waitLiveClient` en paquete `tokenbucket`: 4 llamadas (`limiter_e2e_test.go` ×3, `bench_e2e_test.go` ×1). `windowcounter/` tiene un homónimo propio; este test no lo toca.
- `Redis.Allow` producción: 1 sitio (`tokenbucket/redis.go`). `NewRedis` producción: el constructor; el resto de `NewRedis(` está en tests/benches del mismo paquete.
- `go ` en producción `simpleredis/`: 0. `runtime.SetFinalizer` en dest `tokenbucket/` + `simpleredis/`: not found.

Reproduce: **not reproduced**. Camino ejecutado (worktree de referencia, solo lectura): `TestTraefik13618_ReloadWithoutClose` en `tokenbucket/traefik_issues_e2e_test.go` (no en `traefik_issue13618_e2e_test.go`). Motores `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` y `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380`. Resultado 2026-11-22: PASS 1.63s.

| Motor | Pico sockets | Tras GC | Goroutines antes→después (sin proxy) |
|---|---|---|---|
| redis | 20 | 0 | 4→4 (+0) |
| dragonfly | 20 | 0 | 4→4 (+0) |

Dest no contiene ese test (`TestTraefik13618` / `countingProxy`: not found). El `Allow` de referencia es de tres valores; dest ya es `(Status, error)` — el repro de referencia no compila en dest sin transcribir.

Hechos de fuera: issue https://github.com/traefik/traefik/issues/13618 y PR https://github.com/traefik/traefik/pull/13619 (la existencia del defecto se toma de ahí). C3 en el análisis de referencia `TRAEFIK-UPSTREAM-ISSUES.md`. En árbol: `knowledge/devdocs/std_go_simpleredis.md` (`New` no marca goroutine; sin `Close` un cliente quieto conserva ociosos), `std_go_tokenbucket.md` (el limitador no cierra el cliente), `std_go_test-suites.md` (gate e2e). Research `ext_go-redis_*` justifica por qué no importamos go-redis; no hace falta un finding nuevo de `CircuitBreakerManager.cleanupLoop` para este ticket.

## Decisions

- Costura: test live autocontenido en `tokenbucket/traefik_issue13618_e2e_test.go`; 50 recargas `simpleredis.New` + `NewRedis` + un `Allow` + drop sin `Close`; proxy TCP que cuenta `open` sin retención de escrituras; gate igual que `TestLive_Limiter`; helpers propios con sufijo `13618`.
- Transcribir el repro de referencia al `Allow` dest `(Status, error)` y al umbral `leakedGoroutines > 2` (hoy medido +0).
- Sin cambio de producto: ni `Close` en el limitador, ni reaper/finalizer en `simpleredis`, ni `reclaim/` para compartir el cliente.
- Rechazado: copiar `tokenbucket/traefik_issues_e2e_test.go` (mezcla otros issues).
- Rechazado: restaurar `Allow` de tres valores.
- Rechazado: copiar `holdWrites` del proxy de #14173.
- Rechazado: inventar un control invertido que fuerce la fuga de Traefik (aquí no ocurre).
- Live contract: `std_go_simpleredis_tcp-session` (`New` MUST NOT start a goroutine; idle hasta `Close` o el siguiente borrow), `std_go_tokenbucket_allow` (`Allow` → `Status, error`), `std_go_ci_test-suites` (e2e-redis / e2e-dragonfly; Unit `-short` salta live), `std_go_simpleredis_live-e2e` (skip live). Este cambio no los sustituye; propose puede añadir un escenario de sockets proxied a 0 tras GC si el catálogo lo pide.

## Open questions

- Q: ¿Se conserva el umbral `leakedGoroutines > 2` del repro o se ajusta en dest?
  Rank: additive asked — Tests pide conservar casos de control; Unknowns deja el umbral a implement al transcribir
  Decision: assumed — transcribir `leakedGoroutines > 2`. Hoy el repro midió +0. Implement solo lo aprieta con evidencia dest; no lo afloja.
  By: explore

- Q: ¿Este cambio añade un escenario de catálogo (sockets de clientes abandonados tras GC) o solo el test contra el live contract existente?
  Rank: additive asked — Desired es test only; el contrato live ya cubre «New MUST NOT start a goroutine»
  Decision: assumed — propose transcribe un change de test. Si el catálogo necesita un escenario, es aditivo sobre `std_go_simpleredis_tcp-session` (síntoma proxied=0), sin reescribir el MAY de idle-hasta-Close para un cliente aún alcanzable.
  By: explore

- Q: ¿Sigue dest HEAD pasando la aserción live del repro de referencia (50 recargas, GC, `proxy.open == 0`, goroutines residuales ≤ 2)?
  Rank: additive asked — el test es una unidad que este cambio crea; Desired nombra esa aserción
  Decision: resolved — not reproduced. El repro de referencia pasó hoy contra Redis y Dragonfly (tablas en Concepts). Dest aún no tiene el test; implement lo transcribe con la firma dest de `Allow`.
  By: explore

- Q: ¿El test afirma el mecanismo (finalizer de `netFD` de la stdlib) o el síntoma (sockets proxied a 0)?
  Rank: additive asked — Design «After GC no proxied socket stays open»; Unknowns dice que el cierre del fd es de la stdlib
  Decision: resolved — afirmar `proxy.open == 0` tras GC. No afirmar finalizers ni `netFD`.
  By: explore

- Q: ¿Hay cambio de producto (Close en el limitador, reaper/finalizer, cliente vía reclaim)?
  Rank: additive asked — Design «No product change»; Out of scope nombra Close, reaper y reclaim
  Decision: resolved — no. Propose transcribe; no rediseña.
  By: explore

- Q: ¿Fichero propio `traefik_issue13618_e2e_test.go` o copiar el fichero compartido de referencia?
  Rank: additive asked — Affected nombra el fichero propio; Out of scope prohíbe copiar tests de otros issues
  Decision: resolved — fichero propio. Helpers `13618` en ese paquete. No copiar `traefik_issues_e2e_test.go`.
  By: explore

- Q: ¿Qué firma de `Allow` usa el test nuevo?
  Rank: additive asked — el test es nuevo; Tensions «Transcribir, no restaurar la firma antigua»; dest `tokenbucket/redis.go` ya es `(Status, error)`
  Decision: resolved — `status, err := limiter.Allow(...)`. No restaurar tres valores.
  By: explore

- Q: ¿El proxy de conteo lleva `holdWrites`?
  Rank: additive asked — Design «Keep only the proxy features this test needs (no write hold)»
  Decision: resolved — solo cuenta sockets abiertos. Sin retención de escrituras.
  By: explore

- Q: ¿Se inventa un caso de control invertido que falle como Traefik?
  Rank: additive asked — Tests «keep the control cases»; Tensions: este issue no se reproduce y el repro no tiene control invertido
  Decision: resolved — no. El test afirma el comportamiento correcto (sockets 0, sin goroutines de cliente).
  By: explore

- Q: ¿Se reutilizan `runForEachLiveEngine` / `waitLiveClient` dest o se copian helpers compartidos entre PRs?
  Rank: additive asked — Tests «via the existing runForEachLiveEngine/waitLiveClient»; Out of scope prohíbe ficheros helper compartidos entre PRs
  Decision: resolved — llamar a los helpers dest (2 + 4 call sites enumerados en Concepts). Proxy y clave viven en el fichero del issue con sufijo `13618`.
  By: explore
