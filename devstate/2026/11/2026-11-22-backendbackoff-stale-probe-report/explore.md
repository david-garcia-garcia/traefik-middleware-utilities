# Explore

## Concepts

Unidades que este cambio tocaría:

- **Gate** — `backendbackoff/gate.go` — puerta in-memory por clave: `Config`, `New`, `Close`, `SetNowForTest`, mapa `keys`.
- **memEntry** — `backendbackoff/gate.go:65-74` — crédito, `n`, `state`, `openUntil`, `closedSince`, `probeUntil`, `probeOutstanding bool`. No hay generación, token ni id de lease.
- **Allow / Report** — `backendbackoff/allow.go` — `Allow(key)` admite; `Report(key, success)` anota el resultado. En HALF-OPEN el primer `Report` con `probeOutstanding` se toma como sonda.
- **Pruebas del paquete** — `backendbackoff/zzz_gate_test.go`, `zzz_gate_yaegi_test.go`, `zzz_bench_test.go` — únicos callers Go in-tree de `Gate.Allow` / `Gate.Report`.
- **Contrato vivo cooldown** — `openspec/specs/std_go_backendbackoff_cooldown/spec.md` — la matemática de cooldown usa solo el resultado de la sonda.
- **Contrato vivo allow** — `openspec/specs/std_go_backendbackoff_allow/spec.md` — admisión, crédito saturante, clave opaca del caller; firma actual `Report(key, success)`.
- **Uso** — `knowledge/devdocs/std_go_backendbackoff.md` — `Allow(key)` luego `Report(key, ok)`. No menciona identidad del intento.

Cómo se conectan:

```
CLOSED --B fallos--> OPEN --espera--> HALF-OPEN
  |                    ^                  |
  Allow admite         probe fail         Allow admite 1 sonda
  Report = crédito     Report OPEN        primer Report gana
                       se ignora          (sin id de intento)
```

Call sites de `Gate.Allow` / `Gate.Report`: **32 Allow** y **18 Report** (sitios sintácticos `gate.Allow(` / `gate.Report(`). Raíces: worktree `*.go` para esas llamadas y para el identificador `backendbackoff`; `e2e/` (ningún plugin importa el paquete); `README.md` (snippet, no Go). 0 callers de producción fuera de `backendbackoff/`. El helper `trip` en `zzz_gate_test.go:420` es un Allow+Report reutilizado. El snippet Yaegi `tripprobeSrc` cuenta 2 Allow y 1 Report. Callers fuera del repo: no medidos (Unknowns).

Reproduce: `go test -count=1 -run TestScale_StaleReportDecidesProbe ./backendbackoff` desde `D:\repositories\traefik-middleware-utilities-2026-11-22-backendbackoff-stale-probe-report` sobre una copia throwaway `backendbackoff/zzz_scratch_stale_probe_repro_test.go` (borrada después). **FAIL.** Gist: `after real probe FAILED: state=0 credit=4 n=0 (stateClosed=0)` y `BUG: gate is CLOSED with credit left although the real probe failed`. `state=0` es CLOSED. Cobertura lease caducado: `TestAllow_LostProbeLeaseAdmits` admite la sonda de reemplazo y no afirma el `Report` tardío de la anterior.

Hechos de fuera: research `ext_traefik_plugins_yaegi-generics` (un tipo genérico de otro paquete no se puede nombrar a nivel de paquete bajo Yaegi v0.16.1; tipos concretos del mismo paquete sí). In-tree: `allow.go`, `gate.go`, spec cooldown. No se escribió carpeta de research: el tipo de identidad bajo Yaegi ya está cubierto.

## Decisions

- Chosen seam / approach: deferred — tech lead writes propose; options listed, none chosen.
- Permiso opaco / generación que `Allow` devuelve y `Report` recibe (scale-hunt §4) — en HALF-OPEN solo cuenta el permiso de la sonda activa; el resto se trata como era CLOSED ignorada. Evidencia: `scale-bug-report.md` §4 Dirección de arreglo; `allow.go` hoy no lleva identidad.
- Otras formas de identidad vistas in-tree — (a) generación en `memEntry`: no existe; solo `probeOutstanding` y `probeUntil` (`gate.go:65-74`). (b) token opaco / `Permit`: no hay ese tipo en `backendbackoff`; `tokenbucket.Allow` es cupo de rate limit, otro dominio. (c) lease id: `probeUntil` es un plazo, no un id; `reclaim` usa context-lease / incarnation para el valor guardado, no para `Report`; `traefikemulator` «generation» cancela una config Traefik. Evidencia: esos paths; ninguna es dueña de un intento Allow→Report.
- Reports que no son la sonda: no-op frente a crédito de era CLOSED — HALF-OPEN con `probeOutstanding` false ya es no-op (`allow.go:60-62`); el crédito saturante vive solo en `stateClosed` (`:74-89`). El spec cooldown prohíbe usar esos Reports para `n`/estado; no dice si el éxito/fallo aplica aritmética CLOSED.
- Live contract: `std_go_backendbackoff_cooldown` (probe result only for cooldown math).

## Open questions

- Q: ¿Qué forma de identidad une Allow con Report (generación, token opaco, otro)?
  Rank: bounded incidental — cambia el contrato Allow/Report que ya tiene 32 llamadas a Allow y 18 a Report (raíces: worktree `*.go` `gate.Allow(`/`gate.Report(`, identificador `backendbackoff`, `e2e/`); ningún criterio nombra generación vs token (Unknowns; Out of scope «Elegir el diseño»)
  Decision: resolved — by propose, pointing to design.md
  By: propose

- Q: ¿Un Report de era CLOSED que llega en HALF-OPEN aplica aritmética CLOSED o es no-op?
  Rank: additive incidental — rama nueva en Report; Desired y spec cooldown dicen que no puede cerrar/reabrir ni cambiar `n`, no nombran crédito
  Decision: resolved — by propose, pointing to design.md
  By: propose

- Q: ¿Cuál es el radio de un cambio de firma fuera de este árbol?
  Rank: bounded asked — 32 Allow + 18 Report in-tree enumerados y migrables aquí; Desired «The API may change (breaking is acceptable… no parallel legacy methods)»
  Decision: resolved — by propose, pointing to design.md
  By: propose

- Q: ¿Qué implicaciones Yaegi tiene el tipo de identidad elegido?
  Rank: additive asked — Constraints: Yaegi-safe only; tipo exportado nuevo
  Decision: resolved — tipo concreto del mismo paquete o entero (`uint64`/`int`) carga bajo Yaegi v0.16.1; no exportar una instanciación genérica de otro paquete (`ext_traefik_plugins_yaegi-generics`); no `select` con canales por llamada (Constraints)
  By: explore

- Q: ¿Hay que probar latencia mayor que el cooldown y miles de inflight, más allá del repro?
  Rank: additive incidental — Unknowns lo nombra; ningún criterio pide ese banco; Desired pide el repro y el caso de lease caducado
  Decision: resolved — by propose, pointing to design.md
  By: propose

- Q: ¿Quién es dueño de la identidad del intento (permiso), no de la clave HTTP?
  Rank: additive asked — el trabajo emitiría identidad de intento; `std_go_backendbackoff_allow` ya dice que el caller posee la clave y la librería no lee HTTP
  Decision: resolved — Gate emite la identidad en un Allow admitido; el caller la guarda y la pasa a Report; no se reconstruye desde dirección, usuario, tenant u Host
  By: explore

- Q: ¿Allow devuelve identidad también en deny, o solo si admite?
  Rank: additive incidental — valor de retorno extra en la rama deny; ningún criterio lo nombra
  Decision: resolved — by propose, pointing to design.md
  By: propose

- Q: El paquete de uso no dice qué Report cuenta como sonda; ¿se reescribe ahora?
  Rank: additive incidental — hueco de usage; produce attended espera; el packet describe el contrato actual
  Decision: resolved — by propose, pointing to design.md
  By: propose
