# Explore

## Concepts

- Units this change would touch:
  - `OpenTyped` — `reclaim/opentyped.go` — llama `OpenWithHooks` y hace `stored.(T)`; en mismatch devuelve `zero T` y el error `reclaim: open %q: want %T, got %T` sin soltar el holder.
  - `OpenWithHooks` / `put` / `finishBind` / `dropWhenDone` / `drop` / `releaseHolder` — `reclaim/table.go` — bind de holder, `context.AfterFunc` hacia `drop`, recuento `slot.holders`.
  - `Peek` — `reclaim/table.go` — lee el valor almacenado sin bind (opción de comprobar el tipo en una clave ya publicada).
  - Tests de control — `reclaim/zzz_opentyped_test.go` (`TestOpenTypedTypeMismatchReturnsZeroT`, camino de éxito) y `reclaim/zzz_yaegi_test.go` (`TestYaegi_OpenTypedCallExpressionReturnsT`).
- How they connect:

```
OpenTyped[T]
    -> OpenWithHooks
        -> lookupOpen   (holders++ en bind/wake; reserva en create)
        -> put | wakeAndBind | finishBind
              -> dropWhenDone
                    -> context.AfterFunc(ctx, drop)   [stop no se guarda]
    -> stored.(T)
        ok   -> return typed
        fail -> return zero, error    [hoy: sin drop]
```

- Call sites that matter (no inventados; Traefik plugins fuera de este repo no se pueden enumerar):
  - `OpenTyped`: raíces `**/*.go` con `OpenTyped` y `OpenTyped[`. Definición 1 (`reclaim/opentyped.go:16`). Callers de producto en este repo: 0. Expresiones de llamada in-tree: 6, todas test — `reclaim/zzz_opentyped_test.go` (4: éxito, dos de identidad, mismatch) y `reclaim/zzz_yaegi_test.go` `RunOpenTyped` (2). `e2e/reclaimprobe/plugin.go` usa `table.Open`, no `OpenTyped`.
  - `OpenWithHooks`: raíces iguales. Callers de producto: 2 — `OpenTyped` (`opentyped.go:18`) y el wrapper `Open` (`table.go:389`). El resto está en `reclaim/zzz_*.go`.
  - `context.AfterFunc` en `reclaim/`: 2 — `table.go:558` (`dropWhenDone`, no guarda `stop`) y `alias.go:117` (`dropWatcher`).
  - `Table.Peek`: definición `table.go:446`; callers in-tree solo `reclaim/zzz_peek_test.go`.
- Reproduce: copiado temporalmente `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\reclaim\zzz_scale_hunt_test.go` a `reclaim/zzz_scale_hunt_test.go` (package `reclaim`), ejecutado `go test -count=1 -v ./reclaim -run 'TestScale_OpenTypedMismatchLeaksHolder|TestOpenTypedTypeMismatchReturnsZeroT'` desde el worktree, luego borrado el fichero temporal. **reproduced** — `TestScale_OpenTypedMismatchLeaksHolder` FAIL: `holders=100 closeRuns=0`. `TestOpenTypedTypeMismatchReturnsZeroT` PASS (solo aserta zero + texto de error; no detecta la fuga).
- Outside facts used: research `ext_traefik_plugins_yaegi-afterfunc` (`context.AfterFunc` existe bajo Yaegi v0.16.1; `Done()==nil` no dispara `f`; el símbolo es `func(Context, func()) func() bool`). Official Go 1.21: `AfterFunc` devuelve `stop func() bool`. In-tree: `simpleredis/resp.go` `watchConnClose` sí guarda `stop`; `reclaim/table.go` `dropWhenDone` lo descarta.

## Decisions

- Chosen seam / approach: deferred to tech-lead propose. Esta fase no elige el mecanismo ni el seam del arreglo.
- Listed options (alternativas, con evidencia; ninguna elegida):
  - Soltar el holder tras el mismatch: `OpenTyped` está en el mismo package que `drop` / `releaseHolder`; `finishBind` ya hace `drop` cuando `ctx.Err()` está set (`table.go:537-540`). Tras `OpenWithHooks` el `*slot` no vuelve al caller — habría que recuperar la encarnación (p. ej. `items[key]`) o ampliar el camino interno. `dropWhenDone` registra `AfterFunc` sin `stop` (`table.go:558`): un `drop` ahora y el `AfterFunc` al cancelar `ctx` volvería a `releaseHolder` sobre la misma encarnación.
  - Comprobar el tipo antes de bind en una clave ya publicada: `Peek` entrega el `any` sin `holders++` (`table.go:446-469`). No espera, no hace Wake, `ok=false` si busy/missing. El primer `OpenTyped` (create) no puede usar Peek: el tipo solo existe después de `create`/`publishPut`, y `put` siempre termina en `finishBind`.
  - Híbrido: Peek-then-skip en mismatch de clave viva; rollback de holder en el create/bind que ya publicó. Sigue el problema del `AfterFunc` sin `stop` en el segundo brazo.
- Rejected this phase (alcance, no diseño de producto):
  - Cambiar `OpenTyped` cuando el tipo coincide — Out of scope.
  - Relajar restricciones Yaegi de `opentyped.go` — Out of scope.
  - Reescribir `OpenWithHooks` / `drop` para fallos que no sean el type-assert de `OpenTyped` — Out of scope.
  - Helpers de test compartidos entre PRs paralelos — Out of scope.
- Live contract: `openspec/specs/std_go_reclaim_value-lifecycle` — Requirement `OpenTyped returns the stored value as T`, Scenario `OpenTyped type mismatch returns zero T` (zero + forma del error; no exige soltar holder ni Close). `openspec/specs/std_go_reclaim_context-lease` — bind/drop de holders en `Open` (no nombra el mismatch de `OpenTyped`). Usage `knowledge/devdocs/std_go_reclaim.md` Language **OpenTyped** tampoco menciona release en mismatch.

## Open questions

- Q: ¿Qué mecanismo cumple el efecto (soltar el holder tras el assert, comprobar el tipo antes de bind, o un híbrido)?
  Rank: additive incidental — las tres opciones caben en la ruta de mismatch de `OpenTyped` (firma pública igual; 0 callers de producto in-tree); Out of scope dice que elegir el mecanismo no es un requisito de producto extra
  Decision: assumed — deferred to tech-lead propose; esta fase lista las alternativas con evidencia y no elige seam.
  By: explore

- Q: Si propose suelta el holder después de bind, ¿cómo evita un segundo `drop` cuando el `AfterFunc` ya registrado dispare al cancelar `ctx`?
  Rank: bounded incidental — `dropWhenDone` ya tiene callers (todo bind con `Done != nil` vía `finishBind`); 2 registros `context.AfterFunc` enumerados en `reclaim/` (`table.go:558`, `alias.go:117`); el criterio pide el efecto, no este detalle de stop
  Decision: assumed — el arreglo no puede hacer `releaseHolder` dos veces por el mismo bind; `AfterFunc` oficial devuelve `stop func() bool` y `dropWhenDone` hoy lo descarta (`table.go:558`); `simpleredis/resp.go` `watchConnClose` sí lo guarda.
  By: explore

- Q: En el primer `OpenTyped` cuyo `create` ya publicó un valor de tipo distinto, ¿«como si no hubiera hecho bind» deja la encarnación creada (y Close a grace 0 si era el último holder) o exige no almacenar el valor?
  Rank: additive asked — Desired nombra «had not bound», holder count sin cambio, y «zero-grace Close runs when no other holder remains»; el spec live de create-once prohíbe crear un valor y descartarlo
  Decision: assumed — create ya corrió; el efecto es no dejar este holder. Si no queda otro holder, Close a grace 0. No deshacer `create` como si la clave nunca se hubiera registrado.
  By: explore

- Q: ¿El spec live de mismatch debe pasar a exigir holder sin bind / Close a grace 0, además de zero T y el texto de error?
  Rank: additive asked — Desired nombra ese efecto; el catálogo `std_go_reclaim_value-lifecycle` Scenario `OpenTyped type mismatch returns zero T` hoy no lo exige
  Decision: assumed — propose escribe el delta (MODIFIED en ese requirement, y en `std_go_reclaim_context-lease` solo si el contrato de holder lo necesita); esta fase no escribe `openspec/changes/`.
  By: explore

- Q: ¿La frase de usage **OpenTyped** en `std_go_reclaim.md` debe decir que un mismatch no deja holder?
  Rank: additive incidental — sería una frase de usage/gotcha; ningún criterio pide escribir usage en explore
  Decision: assumed — hueco aparcado; produce attended exige yes humano; esta fase no escribe `knowledge/devdocs`.
  By: explore

- Q: ¿Los tests nuevos van en un `zzz_` propio con sufijo del slug, o se amplía `TestOpenTypedTypeMismatchReturnsZeroT`?
  Rank: additive asked — Constraints: «New test file(s) for this ticket are self-contained; helpers they need are suffixed with this ticket's slug»
  Decision: assumed — fichero `zzz_` nuevo, self-contained, identificadores con sufijo `2026-11-22-reclaim-opentyped-holder-leak`; se conserva el test actual como control de zero + texto de error.
  By: explore

- Q: ¿Hace falta una forma genérica nueva (fuera de la call expression actual de `OpenTyped[T]`) y hay que medirla bajo Yaegi v0.16.1?
  Rank: additive asked — Constraints: Yaegi-safe only; no package-level var/alias/field de una instanciación genérica de otro package
  Decision: assumed — se mantiene `OpenTyped` como call expression; no se añade instanciación genérica a nivel de package. Si propose introduce otra forma genérica, se mide con el harness Yaegi existente antes de darla por válida.
  By: explore

- Q: ¿Plugins Traefik fuera de este repo que llamen `OpenTyped` con tipos distintos sobre la misma clave cambian de contrato público?
  Rank: additive incidental — callers fuera de este repo no se pueden enumerar; Desired deja el `OpenTyped` con éxito igual y no pide cambiar la firma ni el texto de error
  Decision: assumed — firma y texto `reclaim: open %q: want %T, got %T` se quedan; el cambio es el efecto de holder/Close en mismatch. Plugins externos no enumerables.
  By: explore
