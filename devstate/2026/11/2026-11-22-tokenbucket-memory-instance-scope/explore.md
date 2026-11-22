# Explore
IssueKey: 2026-11-22-tokenbucket-memory-instance-scope

## Concepts

Unidades que este cambio toca (nombre, ruta, trabajo):

- **Memory** — `tokenbucket/memory.go`. Un valor, un `map` (`buckets`). `NewMemory` asigna un mapa nuevo. GoDoc hoy no dice que el valor sea el espacio de claves.
- **Redis** — `tokenbucket/redis.go`. `Allow` hace `Eval` sobre la `key` opaca del llamante. El estado vive en el servidor. GoDoc hoy no dice que el prefijo defina el ámbito.
- **Status** — `tokenbucket/status.go`. Dest `Allow` es `(Status, error)`, no el triple `(bool, wait, error)` del repro de referencia.
- **Allow contract** — `openspec/specs/std_go_tokenbucket_allow/spec.md`. Promesa viva: admit/wait/deny, prefijo de `key` del llamante, identidad HTTP del llamante. No exige ni prohíbe compartir un `*Memory`.
- **Usage packet** — `knowledge/devdocs/std_go_tokenbucket.md`. Ya afirma share Redis y «`NewMemory(...)` once». No afirma el aislamiento por instancia de `*Memory` (dos valores = `2*burst`).
- **Live helpers** — `tokenbucket/limiter_e2e_test.go` (`runForEachLiveEngine`, `waitLiveClient`, `TestLive_Limiter` → `twoInstancesShare`). Solo uso, no mover.
- **reclaim.Table** — `reclaim/table.go`. Se cita como medio para reutilizar un `*Memory` entre recargas. Este cambio no lo edita.
- **Debt note** — `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md` + `devstate/issues.md`. El arreglo a nivel middleware queda fuera de la librería.

```
  llamante (middleware; no está en este repo)
       │
       ├─ NewMemory() ──► *Memory.buckets (mapa propio)
       │                      │
       │                      └─ Allow(ctx, key)  consume solo en ese mapa
       │
       ├─ un *Memory compartido (p. ej. reclaim por nombre de middleware)
       │                      └─ mismo mapa → un burst
       │
       └─ NewRedis(client) ──► Allow(ctx, key) ──► HASH Redis en `key`
                                  cualquier *Redis sobre la misma clave comparte
```

Call sites que importan: `NewMemory(` / `NewRedis(` / `tokenbucket.NewMemory` / `tokenbucket.NewRedis` en todo el worktree. `NewMemory(`: 38 coincidencias (1 definición `tokenbucket/memory.go:39`, 37 llamadas, todas en `tokenbucket/*_test.go`). `NewRedis(`: definición `tokenbucket/redis.go:23` más tests del mismo paquete. Ningún otro paquete de producción importa `tokenbucket` (raíces: `**/*.go`, `README.md`, `knowledge/devdocs/`). `traefikemulator` recibe `middlewareName` y no construye un limitador. Consumidores fuera de este repo no se enumeran; este cambio no altera el comportamiento, así que no hay migración.

Reproduce: medido en dest.

- Throwaway fuera del worktree (`repro13706`, `replace` → este árbol): dos `NewMemory`, misma `key`, `burst=3`, `maxDelay=0` → `separate_total=6`; un `*Memory` compartido → `shared_total=3`.
- `go test -short -count=1 -run TestRedis_TwoInstancesShareBurst ./tokenbucket/` → PASS.
- `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 go test -count=1 -run TestLive_Limiter ./tokenbucket/` → PASS, incluidos `redis/twoInstancesShare` y `dragonfly/twoInstancesShare`.
- El repro de referencia `TestTraefik13706_MemoryInstancesPerRouter` no compila contra dest (`Allow` devolvía `(bool, wait, error)`). Los números coinciden con el throwaway (6 frente a burst 3). Ese test espera `total == burst` y por eso falla; el diseño vinculante pide afirmar `2*burst` en (b).

Outside facts: `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (reloj Traefik, prefijo Redis `rate:` + source). Lectura de apoyo, no escrita: worktree de referencia `tokenbucket/TRAEFIK-UPSTREAM-ISSUES.md` sección B2 (#13706) — Traefik construye un limitador in-memory por router (`BuildMiddlewareChain`) y en Redis usa `rate:<middleware>:<source>`. Issue https://github.com/traefik/traefik/issues/13706 ; PRs https://github.com/traefik/traefik/pull/13823 (abierto) y https://github.com/traefik/traefik/pull/13707 (cerrado). No hace falta un finding nuevo: el brief ya cita esos orígenes y B2 responde el hecho de terceros.

## Decisions

- Sin cambio de comportamiento. Transcribir el diseño vinculante: GoDoc en `Memory`/`NewMemory` y `Redis`/`NewRedis`; tests que afirman el contrato; nota de deuda del consumidor ya creada.
- Costura: documentar y testear el ámbito en `tokenbucket/`, no unificar el mapa de `*Memory` ni prefijar `rate:` dentro del paquete (Out of scope).
- Tests nuevos autocontenidos: `tokenbucket/traefik_issue13706_test.go` ((a) y (b)) y `tokenbucket/traefik_issue13706_e2e_test.go` ((c)). Helpers con sufijo `13706`. Reutilizar `runForEachLiveEngine` / `waitLiveClient`. No copiar `traefik_issues_*.go`. API dest: `Status`.
- Dejar `TestLive_Limiter` / `twoInstancesShare` y `TestRedis_TwoInstancesShareBurst` como controles. No extraer helpers compartidos.
- Catálogo vivo: plegar un requisito ADDED en `std_go_tokenbucket_allow` (espacio por instancia Memory, share por clave Redis). No crear una familia nueva. No tocar `std_go_tokenbucket_lua-eval` ni `std_go_tokenbucket_status`.
- Usage: completar el gotcha de `std_go_tokenbucket.md` con el aislamiento por instancia de `*Memory`. Language sin términos nuevos. README ya dice «One map per limiter instance» y share Redis; no reescribirlo.
- `reclaim/` no se edita. Se nombra en GoDoc como ejemplo de cómo compartir un `*Memory`.
- Rechazado: arreglo tipo Traefik `scope: router | middleware` (#13823) dentro de esta librería — Out of scope; la nota de deuda ya lo registra.
- Rechazado: mapa de proceso, constructor unlimited, store global, tocar `simpleredis` / `windowcounter` / `backendbackoff`.
- Live contract: `std_go_tokenbucket_allow` (más el ADDED de ámbito). No `none`.

## Open questions

- Q: ¿El catálogo vivo necesita un requisito de ámbito (Memory por instancia, Redis por clave) o basta GoDoc + tests?
  Rank: additive asked — Desired «El ámbito pasa a ser contrato documentado»; binding «Make the scope part of the documented contract»; Affected nombra `openspec/specs/std_go_tokenbucket_allow/spec.md`; el delta añade una promesa, los callers existentes siguen valiendo
  Decision: assumed — ADDED en `std_go_tokenbucket_allow`: cada `*Memory` es su propio espacio de claves; cualquier `*Redis` sobre la misma `key` comparte el estado del servidor; el prefijo sigue siendo del llamante. GoDoc es la superficie encargada; el catálogo registra la misma promesa. No nueva familia.
  By: explore

- Q: ¿Redacción exacta del GoDoc de Memory/NewMemory y Redis/NewRedis?
  Rank: additive asked — Desired nombra el contenido de esos cuatro GoDoc; no cambia callers
  Decision: assumed — transcribir los bullets del brief: cada `*Memory` es su espacio de claves; para compartir presupuesto entre routers o recargas, compartir un `*Memory` (p. ej. `reclaim` claveado por nombre de middleware); Redis: el estado vive en el servidor, cualquier limitador sobre la misma clave lo comparte, el prefijo define el ámbito. La cita `traefik/traefik#13706` va en el comentario del subtest (b), no hace falta en el GoDoc.
  By: explore

- Q: ¿(a) y (b) viven en el mismo test o en subtests?
  Rank: additive asked — Tests pide reescribir el repro de referencia; Desired nombra (a) y (b) y el fichero `tokenbucket/traefik_issue13706_test.go`
  Decision: assumed — un fichero, dos subtests (`sharedMemory` burst total; `separateMemory` `2*burst` con comentario que cita #13706). Helper `countAdmits13706` en ese fichero. No un test que falle.
  By: explore

- Q: ¿Cómo nombrar y acotar `liveKey13706` para no chocar con `t.Name()` de `TestLive_Limiter` ni con PRs paralelos?
  Rank: additive asked — Tests nombra helpers con sufijo del issue (`liveKey14297`, `countAdmits14297`) y prohíbe ficheros compartidos
  Decision: assumed — `liveKey13706` en `traefik_issue13706_e2e_test.go`: nonce de run + `t.Name()` + sufijo, prefijo distinto de las claves `t.Name()` de `TestLive_Limiter`. `countAdmits13706` en el fichero unitario, lee `Status.Allowed`. Gate: mismos `runForEachLiveEngine` / `waitLiveClient` / env `TOKENBUCKET_LIVE_*`.
  By: explore

- Q: ¿Quién ya posee la identidad (dirección de cliente, usuario, tenant, Host, hop de confianza) y el nombre de middleware con el que se compartiría un `*Memory`?
  Rank: additive incidental — ningún criterio pide reconstruir identidad; el spec vivo ya dice que el llamante posee la clave; se registra para que implement no añada un builder
  Decision: assumed — ninguno en esta librería. Reutilizar la `key` que pasa el llamante (`std_go_tokenbucket_allow`: «Prefixing of `key` is the caller's job», «Callers own identity; the library never reads HTTP»). El nombre de middleware como clave de `reclaim` también es del consumidor. No reconstruir Host ni sourceCriterion aquí.
  By: explore

- Q: ¿El middleware consumidor de esta librería ya comparte un `*Memory` vía `reclaim` claveado por nombre de middleware?
  Rank: additive asked — Desired pide la nota en `devstate/issues.md` + `knowledge/debt/`; Out of scope nombra el arreglo de middleware
  Decision: resolved — not found en dest: ninguna llamada de producción a `NewMemory` fuera de `tokenbucket/*_test.go`; `traefikemulator` no importa `tokenbucket`. La nota `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md` ya existe (prepare). No implementar el cableado del consumidor.
  By: explore

- Q: ¿Hay que completar el gotcha de `knowledge/devdocs/std_go_tokenbucket.md` con el aislamiento por instancia de `*Memory`?
  Rank: additive asked — Affected nombra ese packet; el usage actual afirma el share Redis y omite el hecho simétrico de Memory
  Decision: assumed — implement actualiza el gotcha (dos `*Memory` admiten `2*burst`; compartir un valor para un presupuesto). Language sin cambios. README ya cubre el mapa por instancia; no reescribirlo.
  By: explore

- Q: ¿(a) y (b) deben correr también interpretados (Yaegi)?
  Rank: additive incidental — el brief pide tests autocontenidos compilados y e2e live; el spec «Unit and Yaegi prove Allow» cubre burst-after-idle y refund, no el ámbito de instancia
  Decision: assumed — solo compilado + e2e live. No añadir `traefik_issue13706_yaegi_test.go`.
  By: explore
