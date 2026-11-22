# reclaim: OpenTyped with the wrong type keeps the holder it just bound

## Problem

`OpenTyped` calls `OpenWithHooks`, which binds a holder (and a `context.AfterFunc` on the caller's ctx), and only then type-asserts `stored.(T)`. On mismatch it returns an error without releasing that holder. The caller got no value but its context keeps the incarnation awake: the value cannot sleep or close while that ctx lives, and every retry with the same ctx adds another holder and AfterFunc. Realistic trigger: two consumers share one `Table` and use the same key string with different types (or a programming error).

## Current (code, master 1f5ec454)

- `reclaim/opentyped.go:18-25`.
- Holder bookkeeping and release paths in `reclaim/table.go` (`OpenWithHooks`, drop/release).

## Evidence

100 failed `OpenTyped[int]` on a key whose create returns a string → `holders=100`, Close never runs (Grace 0).
Repro test: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\reclaim\zzz_scale_hunt_test.go` — `TestScale_OpenTypedMismatchLeaksHolder` (white-box, reads `tab.items[k].holders`).

## Desired

A failed type check leaves the table exactly as if that `OpenTyped` had not bound: holder count unchanged, its context no longer keeps the value, zero-grace Close runs when no other holder remains. A successful `OpenTyped` behaves as today. Yaegi constraints on generics in `opentyped.go` stay respected.

## Process note (binding for this run)

- Phases prepare and explore run first. Explore lists the options and open questions; it does NOT pick the final design.
- The propose phase (OpenSpec change) is written by the tech lead, not by the agent that ran explore. Do not start propose on your own.
- Report and evidence for all seven tickets of this batch: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\scale-bug-report.md` (read only the section for this ticket).

## Constraints

- The library runs compiled AND interpreted under Yaegi v0.16.1 (Traefik plugin). Yaegi-safe only: no `select` mixing per-call channels in package source, no `unsafe`, no `container/list` (any-typed Value), no package-level var / type alias / struct field naming a generic instantiation from another package. Existing benchmarks are reported compiled and Yaegi; any performance claim must show both.
- Test files are prefixed `zzz_` (#116). New test file(s) for this ticket are self-contained; helpers they need are suffixed with this ticket's slug so the seven parallel PRs never declare the same identifier. Do not create shared helper files. Tests assert what a correct implementation must do (no tautologies) and keep the control cases.
- Local gate before pushing: `go vet ./...` ; `go test -short -count=1 ./...` ; plus the touched package without `-short`. Local engines in Docker if a live test is needed: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret).
- CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
- Six sibling tickets of this batch run in parallel on other branches. Touch only what this ticket needs. Three of them (`backendbackoff-*`) edit the same package: keep hunks minimal so merges stay mechanical.

## Current (code)

- `reclaim/opentyped.go:16-27` — `OpenTyped` llama `OpenWithHooks`, y solo si ese Open no falla hace `stored.(T)`. Si el assert falla, devuelve `zero` y un error; no llama a `drop` ni a `releaseHolder`.
- `reclaim/table.go:400-434` — `OpenWithHooks` crea (`put`) o reutiliza (`finishBind` / `wakeAndBind`) y siempre termina en un bind de holder.
- `reclaim/table.go:472-492` — `put` publica el valor y llama `finishBind`.
- `reclaim/table.go:535-544` — `finishBind` registra `dropWhenDone` y entrega el puntero; es el punto en el que el holder queda ligado al ctx.
- `reclaim/table.go:554-568` — `dropWhenDone` arma `context.AfterFunc` sobre el ctx del llamante para `drop`.
- `reclaim/table.go:90-98` — `slot.holders` es el contador que el spec mide.
- `reclaim/table.go:580-585` — `releaseHolder` decrementa `holders`.
- `reclaim/table.go:662-677` — `drop` llama `releaseHolder` y, si era el último holder despierto, duerme / cierra la encarnación.
- `reclaim/zzz_opentyped_test.go:144-159` — `TestOpenTypedTypeMismatchReturnsZeroT` aserta valor cero y el texto del error; no aserta `holders` ni que `Close` corra.
- `reclaim/zzz_opentyped_test.go:100-115` — `TestOpenTypedReturnsTypedValue` cubre el camino de éxito (el spec pide dejarlo igual).
- `reclaim/zzz_yaegi_test.go:63-74` — `TestYaegi_OpenTypedCallExpressionReturnsT` cubre la forma de llamada que Yaegi puede resolver.
- En este worktree, las únicas instanciaciones de `OpenTyped` están en esos tests; no hay callers de producto en otros paquetes.
- `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\reclaim\zzz_scale_hunt_test.go` (`TestScale_OpenTypedMismatchLeaksHolder`) — `not found` (el repro no está en este árbol).

## Out of scope

- Los otros seis tickets del mismo batch (otros worktrees / IssueKey).
- Cambiar `OpenTyped` cuando el tipo coincide.
- Relajar las restricciones Yaegi de `opentyped.go`.
- Helpers de test compartidos entre PRs paralelos.
- Reescribir `OpenWithHooks` / `drop` para fallos que no sean el type-assert de `OpenTyped`.
- Elegir el mecanismo del arreglo (soltar el holder vs comprobar el tipo antes de bind): eso lo lista explore; no es un requisito de producto extra.

## Unknowns

- Runtime no medido en este worktree: el recuento `holders=100` y “Close never runs” viene del spec / informe; aquí no se reejecutó.
- Radio de explosión fuera de este repo (plugins Traefik que llamen `OpenTyped` con tipos distintos sobre la misma clave).
- El arreglo concreto: el spec pide el efecto (tabla como si ese `OpenTyped` no hubiera hecho bind); no elige implementación.
- Si el `context.AfterFunc` ya registrado se puede anular, o si el ctx del llamante sigue disparando `drop` más tarde.
- Comportamiento Yaegi v0.16.1 de cualquier forma genérica nueva que explore/propose consideren (no medido aquí).

## Tensions

- Informe §6 ofrece dos direcciones (soltar el holder, o comprobar el tipo antes de registrarlo). El spec pide solo el efecto observable. No hay `RETHINK` en el PR. Explore lista las opciones; no elige.
- `TestOpenTypedTypeMismatchReturnsZeroT` pasa hoy con la fuga: no contradice el bug, tampoco lo detecta.
- El spec ancla Current a master `1f5ec454`; dest HEAD de este worktree es `35aaa343c5185f8d5315e8715c3ceb05caed247a`. No medido si `reclaim/` difiere entre esos SHAs.
- Nota de proceso: propose lo escribe el tech lead; este prepare no abre `openspec/`.
