# Explore

## Concepts

### Unidades que toca el cambio

| Unidad | Ruta | Trabajo hoy |
|---|---|---|
| `tokenbucket.Redis.Allow` | `tokenbucket/redis.go:52-84` | Consume 1 vía `Eval` de `allowScript`. GoDoc (L52-53) no nombra `simpleredis.ErrUnreachable` / `ErrTimeout` ni que fail-open / fail-closed lo decide el middleware. Cuerpo: si `Eval` falla, `return Status{}, err` (L67-69). El Status cero tiene `Allowed=false`. |
| `simpleredis.Eval` | `simpleredis/commands_eval.go:28-35` | Propaga el error de `exec`. No se toca. |
| `simpleredis.ioError` | `simpleredis/resp.go:394-404` | Deadline → `errTimeout` (`ErrTimeout`); resto de I/O (salvo rechazo TLS) → `errUnreachable` (`ErrUnreachable`). Puerto loopback que rechaza cae aquí. No se toca. |
| Centinelas | `simpleredis/simpleredis.go:28-31` | `ErrUnreachable` / `ErrTimeout` exportados; comentario: casar con `errors.Is`. `Error()` sigue siendo `redis:unreachable` / `redis:timeout`. No se toca. |
| Tests existentes de outage | `tokenbucket/limiter_test.go:230-239`, `tokenbucket/status_test.go:164-181` | `TestRedis_Unreachable` y el tramo Redis de `TestStatus_ErrorReturnsTheZeroStatus` apuntan a `127.0.0.1:1` y casan `err.Error() == simpleredis.RedisUnreachable`. El segundo también exige `Status{}`. No usan `errors.Is` ni leen `Allowed` en el primero. Se conservan. |
| Test dedicado (nuevo) | `tokenbucket/traefik_issue12091_test.go` | `not found`. El repro de referencia vive solo en el árbol de lectura `tokenbucket-traefik-upstream-repros` (`TestTraefik12091_RedisUnreachable`). |
| Usage | `knowledge/devdocs/std_go_tokenbucket.md:28` | «Match Redis errors by `Error()` text». El paquete SimpleRedis ya enseña `errors.Is` (`std_go_simpleredis.md`). |
| Contrato vivo | `openspec/specs/std_go_tokenbucket_allow/spec.md` | «Error returns the zero status» (Redis error genérico) y «Redis errors do not become deny» / escenario Unreachable (`redis:unreachable`, no `errors.Is`, y «does not return allowed false as a substitute»). |

No hay reconstrucción de identidad: `Allow` sigue tomando una clave opaca del llamador (`std_go_tokenbucket_allow` Scenario: Caller owns the key). Este cambio no fija client address, user, tenant, Host ni trust hop. No aplica la pregunta de dueño de identidad.

Política HTTP fail-open / fail-closed en `tokenbucket`: `not found` (la librería no elige código HTTP).

### Cómo se conectan

```
middleware (fuera del repo; elige fail-open o fail-closed / 500)
   │  Allow(ctx, key)  →  (Status, error)
   ▼
tokenbucket.Redis.Allow
   │  Eval(allowScript)
   ▼
simpleredis.Eval → exec → ioError
   │  dial refuse / I/O     deadline
   ▼                        ▼
ErrUnreachable           ErrTimeout
   │
   └─ Allow returns Status{}, err
      Allowed=false because Status is zero, not because Allow denied
```

Seam: el GoDoc y un test que fijen el centinela con `errors.Is` y `Allowed=false`. El cuerpo de `Allow` ya es el contrato. Aguas arriba Traefik no ofrece esa opción y contesta 500 ([traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091)).

### Call sites (enumerados)

Raíces: worktree, `*.go` con `NewRedis(` y `\.Allow(`, más `Match Redis errors` en `*.md`. Excluye `devstate/` y `openspec/changes/archive/`.

- Importadores Go de `tokenbucket` fuera del paquete (producción): **0** (`not found` en `e2e/`, plugins, otros paquetes).
- `NewRedis(`: **1** producción (`tokenbucket/redis.go:23`) + tests/benches en `tokenbucket/` (limiter, status, yaegi, e2e, repro_*, allow_*, ttl, bench).
- Casado de outage Redis por `Error()` en `tokenbucket/`: **2** (`limiter_test.go:237`, `status_test.go:178`). Siguen válidos: `Error()` sigue siendo `redis:unreachable`.
- `errors.Is(..., simpleredis.ErrUnreachable)` en `tokenbucket/`: **0** (`not found`). Existe en `simpleredis/errors_test.go` para `ErrPoolWait`.
- Documentación: `knowledge/devdocs/std_go_tokenbucket.md:28` (Error() text); `std_go_simpleredis.md` (errors.Is).
- Llamadores externos (middleware en otro repo): **no enumerables** desde este repo. Este cambio no altera el retorno: los que ya miran `err != nil` siguen viendo el mismo error.

### Reproducción

Afirmación del ticket: la librería **ya** superficie el outage y deja la política al llamador. Medido en este worktree (no el árbol de referencia).

1. Lectura: `Redis.Allow` L67-69 devuelve `Status{}, err` sin fail-open/fail-closed. `ioError` L398-404 mapea I/O a `errUnreachable` (alias de `ErrUnreachable`).
2. Tests existentes en el worktree:

   `go test -short -count=1 -timeout 2m -v -run 'TestRedis_Unreachable$|TestStatus_ErrorReturnsTheZeroStatus$' ./tokenbucket/`

   → `TestRedis_Unreachable` PASS (0.02s); `TestStatus_ErrorReturnsTheZeroStatus` PASS (0.02s).

3. Sonda throwaway (árbol temp, `replace` al worktree, `MaxRetries: -1`, `127.0.0.1:1`):

   ```
   err=redis:unreachable
   errors.Is(err, ErrUnreachable)=true
   errors.Is(err, ErrTimeout)=false
   allowed=false status_zero=true
   ```

Conclusión: **reproducido como «not a library policy defect»**. El 500 de Traefik no ocurre aquí. El hueco es TEST + DOC: fichero dedicado con `errors.Is` + `Allowed=false`, y GoDoc que nombre los centinelas y la decisión del middleware.

`tokenbucket/traefik_issue12091_test.go` y `TestTraefik12091_RedisUnreachable`: `not found` en dest. El repro de referencia (`traefik_issues_test.go:252-270`) usa la firma vieja `(bool, wait, error)` de antes de #106; dest es `(Status, error)`.

### Hechos externos usados

- In-tree: `knowledge/devdocs/std_go_tokenbucket.md`, `std_go_simpleredis.md`, `std_go_test-suites.md`; contrato `openspec/specs/std_go_tokenbucket_allow/spec.md`; requirement cita [traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091) (cerrado aguas arriba como duplicado de #12043; ningún PR upstream cerró #12091).
- Medido en esta ejecución: refuse en `127.0.0.1:1` → `errors.Is(err, ErrUnreachable)` y Status cero. No se escribe research nuevo: el hecho de Traefik no está en duda para el trabajo de esta librería.
- Árbol de referencia (solo lectura, no dest): `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\traefik_issues_test.go` `TestTraefik12091_RedisUnreachable`.

## Decisions

### Elegido

- Sin cambio de comportamiento: el cuerpo de `Redis.Allow` se queda en `return Status{}, err` (Design binding).
- Test: `tokenbucket/traefik_issue12091_test.go` autocontenido, `TestTraefik12091_RedisUnreachable`, helpers con sufijo `12091` en ese fichero, `127.0.0.1:1`, `MaxRetries: -1` como el repro de referencia.
- Afirmaciones: `errors.Is(err, simpleredis.ErrUnreachable)` y `!status.Allowed`. No tautología del Status entero (eso ya lo cubre `TestStatus_ErrorReturnsTheZeroStatus`).
- GoDoc en `Redis.Allow`: en un outage devuelve `simpleredis.ErrUnreachable` (o `ErrTimeout`) con `allowed=false`; fail-open o fail-closed es decisión del middleware; citar [traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091).
- Firma dest: `(Status, error)` — no reintroducir `(bool, wait, error)`.
- Conservar `TestRedis_Unreachable` y el tramo Error() de `TestStatus_ErrorReturnsTheZeroStatus` (casos de control / matching legado).
- Alinear `knowledge/devdocs/std_go_tokenbucket.md` How to use a `errors.Is` (y `IsUnreachable`) contra los centinelas, como `std_go_simpleredis.md`. `Error()` sigue siendo el texto de display.
- Propose: MODIFIED de `std_go_tokenbucket_allow` para nombrar `errors.Is` / `ErrUnreachable` / `ErrTimeout` y `Allowed=false` en el Status cero, sin borrar «does not return allowed false as a substitute» (eso sigue significando: no tragar el error y denegar).
- Control live: no añadir live 12091; `TestLive_Limiter` (`tokenbucket/limiter_e2e_test.go:84-86`, gating `runForEachLiveEngine` / `waitLiveClient`) ya es el camino feliz.
- Sin test `ErrTimeout` en este issue: el Design lo nombra en GoDoc; la sección Tests solo nombra el puerto que rechaza. `simpleredis` ya clasifica deadline → `ErrTimeout`.

### Rechazado

- Cambiar el cuerpo de `Redis.Allow` o envolver el error de Eval (rompería `errors.Is` y el `Error()` exacto que el spec de slog/tcp-session fija).
- Añadir política fail-open / fail-closed o un código HTTP en la librería.
- Reproducir el 500 de Traefik.
- Copiar tests de otros issues o extraer helpers compartidos (`newSimpleRedisForTest` se queda en `limiter_test.go`).
- Cambiar centinelas de `simpleredis`.
- Implementar [traefik/traefik#12043](https://github.com/traefik/traefik/issues/12043).
- Editar el árbol de referencia `tokenbucket-traefik-upstream-repros`.
- Sustituir `TestRedis_Unreachable` por el fichero nuevo.
- Test live contra `TOKENBUCKET_LIVE_*` para un puerto que rechaza.
- Peer colgado para probar `ErrTimeout` en `tokenbucket`.
- Dejar el usage en «Match Redis errors by `Error()` text» como instrucción canónica (contradice el Design y `std_go_simpleredis.md`).

### Contrato vivo

- `openspec/specs/std_go_tokenbucket_allow/spec.md` — live; propose MODIFIED del escenario Unreachable / Error returns the zero status para nombrar los centinelas. No es `none — no live contract`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — dueño de `redis:unreachable` / `redis:timeout`; **sin delta** (Out of scope: no cambiar centinelas).
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — sin delta (el script no cambia).

## Open questions

- Q: ¿Basta el GoDoc de `Redis.Allow` o hay que alinear `knowledge/devdocs/std_go_tokenbucket.md:28` («Match Redis errors by `Error()` text») con `errors.Is`?
  Rank: additive incidental — una línea de usage existente; ningún criterio del Design nombra el usage (Affected lo deja a explore/propose; el pedido nombra GoDoc).
  Decision: assumed — alinear How to use a `errors.Is` / `IsUnreachable` contra `ErrUnreachable` y `ErrTimeout`, igual que `std_go_simpleredis.md`. El snippet `if err != nil { return err }` puede quedarse como un fail-closed de ejemplo; el GoDoc dice que la política es del middleware. No tocar `std_go_windowcounter.md`.
  By: explore

- Q: ¿El spec vivo necesita un escenario que nombre `ErrUnreachable` / `ErrTimeout` (hoy el escenario de error dice «Redis error» y Unreachable casa el texto `redis:unreachable`)?
  Rank: additive incidental — escenario extra o aclaración de un contrato ya vivo; ningún acceptance criterion nombra el spec (Affected: solo si propose añade el escenario).
  Decision: assumed — propose pliega en `std_go_tokenbucket_allow` un escenario que nombra `errors.Is(err, simpleredis.ErrUnreachable)` (y `ErrTimeout` en GoDoc/SHALL existente) con Status cero / `Allowed` false. Conservar «does not return allowed false as a substitute»: el error es el outage, no un deny silencioso. `Error()` sigue siendo `redis:unreachable`.
  By: explore

- Q: ¿Hace falta un test del camino `ErrTimeout` (el Design lo nombra en GoDoc; el test nombrado es solo puerto que rechaza)?
  Rank: additive incidental — test extra no pedido por la sección Tests; Design nombra `ErrTimeout` solo en GoDoc.
  Decision: assumed — no. GoDoc nombra `ErrTimeout`; el fichero 12091 cubre refuse → `ErrUnreachable`. Un peer colgado sería maquinaria nueva. `simpleredis` ya prueba la clasificación timeout.
  By: explore

- Q: ¿Este issue necesita un test live contra Redis/Dragonfly?
  Rank: additive incidental — suite e2e extra; el repro de referencia es unitario contra `127.0.0.1:1`; Tests dice «Si hay tests live» (condicional).
  Decision: assumed — no. Unreachable no necesita motor. Si más tarde hubiera live en ese fichero, el gating sería `runForEachLiveEngine` / `waitLiveClient` como `TestLive_Limiter`. El camino feliz live ya existe.
  By: explore

- Q: ¿Cómo adaptar la firma del repro de referencia (`Allow` devolvía `bool, wait, error`) a `(Status, error)` de dest (#106)?
  Rank: additive asked — Desired Tests mapea `TestTraefik12091_RedisUnreachable`; dest ya cambió la firma en #106 (In-scope / Current).
  Decision: assumed — `status, err := limiter.Allow(...)`; `errors.Is(err, simpleredis.ErrUnreachable)`; `if status.Allowed { t.Fatal(...) }`. No reintroducir la tupla. `wait` no se afirma en outage (Status cero).
  By: explore

- Q: ¿Los tests existentes que casan `err.Error() == RedisUnreachable` se reescriben a `errors.Is`?
  Rank: additive incidental — no se cambia ningún contrato existente; 2 sitios enumerados (`tokenbucket/limiter_test.go:237`, `tokenbucket/status_test.go:178`; raíces `tokenbucket/**/*.go`) se dejan; ningún criterio pide reescribirlos.
  Decision: assumed — no reescribir. El fichero 12091 es la prueba `errors.Is`. El matching por texto sigue siendo cierto y es control legado.
  By: explore
