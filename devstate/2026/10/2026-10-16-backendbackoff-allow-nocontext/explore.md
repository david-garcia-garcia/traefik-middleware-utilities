# Explore

## Concepts

**Unidades**

| Unidad | Ruta | Trabajo |
|--------|------|---------|
| `Gate.Allow` | `backendbackoff/allow.go` | Admisión síncrona por clave: crédito, OPEN/HALF-OPEN/CLOSED, mutex `g.mu`. Hoy rechaza con `ctx.Err()` antes del lock. |
| `Gate.Report` | `backendbackoff/allow.go` | Resultado booleano sin contexto (sin cambios). |
| Tests unitarios | `backendbackoff/gate_test.go` | Cobertura de máquina de estados; incluye `TestAllow_CanceledContext`. |
| Probe Yaegi | `backendbackoff/gate_yaegi_test.go` | Interpreta fuentes no-test y llama `gate.Allow(ctx, "k")`. |
| Benchmark | `backendbackoff/bench_test.go` | `gate.Allow(ctx, "k")` en bucle caliente. |
| Contrato vivo | `openspec/specs/std_go_backendbackoff_allow/spec.md` | Exige `Allow(ctx, key)`, `context.Background()` sin deadline, escenario cancelación. |
| Uso | `knowledge/devdocs/std_go_backendbackoff.md` | Snippet con `gate.Allow(req.Context(), …)`. |

**Conexión**

```text
Caller (middleware) ──► Allow(key) ──► admit/deny + retryAfter
                              │
                              ▼ (si admitió)
                         backend real ──► Report(key, success)
```

La clave opaca y la decisión de llamar tras cancelación del request siguen en el caller; la librería no lee HTTP (`spec.md` escenario caller owns key).

**Call sites de `gate.Allow` (enumerados)**

- Raíz buscada: `backendbackoff/**`, patrón `gate.Allow(`.
- **36 invocaciones** en el paquete: `gate_test.go` (32), `gate_yaegi_test.go` (2), `bench_test.go` (2).
- Definición: `allow.go` — `func (g *Gate) Allow(ctx context.Context, key string) (bool, time.Duration, error)`.
- Fuera del paquete en este árbol (no listados en **Affected** del ticket): `README.md` (1 ejemplo), `knowledge/devdocs/std_go_backendbackoff.md` (1 snippet; **Affected** sí lo nombra).
- No hay importadores Go de `backendbackoff` fuera de tests/docs en el worktree (solo el propio paquete y el string embebido en Yaegi).

**Reproducir firma actual**

- Evidencia en código: `backendbackoff/allow.go` líneas 9–11 — `Allow(ctx context.Context, key string)` y `if err := ctx.Err(); err != nil { return false, 0, err }`.
- Comando: `go test -short -run TestAllow_CanceledContext ./backendbackoff/...` en el worktree — **PASS** (deniega con `context.Canceled` sin admitir).

**Hechos externos**

- In-tree: `knowledge/devdocs/std_go_backendbackoff.md`, spec citada arriba.
- Sin research slug: consumidores Traefik fuera del repo no visibles aquí (`requirement.md` **Unknowns**).

## Decisions

- **Enfoque (una línea):** Cambiar a `Allow(key string)`, quitar el early return por `ctx.Err()` y migrar las 36 call sites + spec + devdocs listados en el ticket; el caller comprueba cancelación antes de llamar.
- **Rechazado:** Mantener `context.Context` en `Allow` (contradice **Desired**); añadir contexto a `Report` (**Out of scope**); alternativas de rendimiento (sharding, `sync.Map`, atomics, `RWMutex`) — explícitamente fuera de alcance de este ticket.
- **Contrato vivo:** `openspec/specs/std_go_backendbackoff_allow/spec.md` (catálogo `openspec/specs/map.md` → `backendbackoff`).

## Open questions

- Q: ¿Existen plugins Traefik u otros consumidores fuera de este repositorio que sigan en `Allow(ctx, key)` y deban migrarse en este PR?
  Rank: additive asked — no reshape en árbol; **Unknowns** en `requirement.md` nombra la incertidumbre sin pedir migración externa
  Decision: assumed — fuera de alcance de este repo; el cambio de firma es breaking a propósito; no bloquear implement; quien despliegue copias externas migra en su propio ciclo.
  By: explore

- Q: ¿Quién debe rechazar o omitir la llamada cuando el contexto del request ya está cancelado?
  Rank: additive asked — **Desired**: «callers decide whether to invoke `Allow` after their request context is done»
  Decision: assumed — el middleware/caller (host Traefik), no `backendbackoff`; reutilizar el flujo habitual de no iniciar trabajo de backend tras cancel; devdocs dejarán de pasar `req.Context()` a `Allow` y documentarán comprobar cancelación antes.
  By: explore

- Q: ¿Qué hacer con `TestAllow_CanceledContext` y la cobertura de cancelación en tests?
  Rank: bounded asked — **Desired** pide actualizar tests y «remove or replace canceled-context coverage»; 1 test + escenario en spec a retirar
  Decision: assumed — eliminar `TestAllow_CanceledContext`; no añadir sustituto que simule middleware en este paquete; la nueva spec declara cancelación pre-`Allow` como responsabilidad del caller (simétrico con `Report` sin contexto).
  By: explore

- Q: ¿`Allow` sigue devolviendo `(bool, time.Duration, error)` con errores distintos de contexto?
  Rank: additive asked — **Desired** no elimina el tercer retorno; **Out of scope** excluye cambiar crédito/mutex/map
  Decision: assumed — conservar errores de gate (`errClosed`, `errUnknown`, etc.); dejar de devolver `context.Canceled` / `DeadlineExceeded` desde `Allow`.
  By: explore

- Q: ¿Alinear `README.md` (ejemplo `gate.Allow(req.Context(), …)`) en la misma entrega?
  Rank: bounded incidental — 1 call site (`README.md` ~123); **Out of scope**: «README.md and other docs not listed in the ticket (unless implement chooses alignment)»
  Decision: assumed — no incluir README en el alcance mínimo; si implement alinea, append en `deviations.md`.
  By: explore
