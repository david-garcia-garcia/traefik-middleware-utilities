# backendbackoff: a stale Report decides the HALF-OPEN probe

## Problem

`Allow` records only a boolean `probeOutstanding`, and `Report(key, success)` carries no identity of the attempt. In HALF-OPEN, the first `Report` to arrive is taken as the probe result, whether it comes from the probe or from a request admitted long before.

Reproduced sequence (TripFailures 5, Jitter 0, BaseCooldown 1s):
1. 6 requests admitted while CLOSED.
2. 5 fail fast → OPEN. Request 6 is still in flight.
3. Cooldown elapses → probe P admitted (HALF-OPEN).
4. Request 6 finishes OK → `Report(true)` is taken as the probe → CLOSED with full credit.
5. Real probe P fails → `Report(false)` only subtracts one credit.
Result: CLOSED, credit 4, n=0 although the real probe failed.

Same when a probe lease expires: a replacement probe is admitted and the late report of the first one takes its slot.

Impact at concurrency: an unhealthy backend is reopened by a stale success (flapping), a healthy one stays blocked by a stale failure and `n` is inflated toward MaxCooldown. Spec `std_go_backendbackoff_cooldown` says cooldown math must use only the probe result.

## Current (code, master 1f5ec454)

- `backendbackoff/allow.go:8-40` `Allow(key)` HALF-OPEN branch (`:30-36`).
- `backendbackoff/allow.go:43-93` `Report(key, success)`, HALF-OPEN branch `:59-73`.

## Evidence

Repro test: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\backendbackoff\zzz_scale_hunt_test.go` — `TestScale_StaleReportDecidesProbe` (fails on master). Ignore the other tests in that file (other tickets).

## Desired

A HALF-OPEN outcome is decided only by the report of the active probe. Reports from attempts admitted earlier (CLOSED era, or an expired probe lease) cannot close or reopen the gate nor change `n`. Repro passes; tests also cover the expired-lease case. The API may change (breaking is acceptable for this owner when it is the clean option; no parallel legacy methods).

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

- `backendbackoff/allow.go` `Allow` rama HALF-OPEN (`:30-36`): si `probeOutstanding` y el reloj está antes de `probeUntil`, niega; si no, pone `probeOutstanding=true`, renueva `probeUntil` a `now+BaseCooldown` y admite. No hay identidad del intento.
- `backendbackoff/allow.go` `Allow` rama OPEN (`:22-29`): al vencer `openUntil` pasa a HALF-OPEN, marca `probeOutstanding=true` y admite la sonda.
- `backendbackoff/allow.go` `Report` (`:43`): firma `Report(key string, success bool)` — no recibe identidad del intento.
- `backendbackoff/allow.go` `Report` rama HALF-OPEN (`:59-73`): si `probeOutstanding` es true, el primer Report lo consume; success cierra con crédito B; failure abre y hace `n++`. Si `probeOutstanding` es false, no-op.
- `backendbackoff/gate.go` `memEntry` (`:65-74`): `probeOutstanding bool` y `probeUntil time.Time`. No hay generación ni token.
- `backendbackoff/zzz_gate_test.go` `TestAllow_LostProbeLeaseAdmits` (`:60-74`): admite una sonda de reemplazo al caducar el lease; no comprueba el Report tardío de la sonda anterior.
- `backendbackoff/zzz_gate_test.go` `TestReport_ProbeSuccessRetainsN` (`:245-285`) y `TestReport_ProbeFailureIncrementsN` (`:287-309`): Reports secuenciales de la sonda; no cubren un Report de una petición anterior.
- `openspec/specs/std_go_backendbackoff_cooldown/spec.md`: cooldown math MUST NOT use Report outcomes other than that probe result.
- Repro `TestScale_StaleReportDecidesProbe`: `not found` en este worktree (vive en el árbol scale-hunt).
- Dest HEAD de este PR: `35aaa343c5185f8d5315e8715c3ceb05caed247a`. El spec cita master `1f5ec454`; `backendbackoff/allow.go` en este worktree coincide con lo descrito.

## Out of scope

- Los otros seis tickets del batch (flush de windowcounter, árbol iplookup, barrido O(n) de claves, jitter en MaxCooldown, reclaim OpenTyped, Sleep concurrente).
- Helpers compartidos entre PRs; tests de otros tickets en el mismo archivo de repro.
- Métodos legacy en paralelo (el spec acepta rotura de API, no duplicar).
- Elegir el diseño: explore lista opciones; propose lo escribe el tech lead.

## Unknowns

- Forma de la identidad (generación, token opaco, otro): explore lista, no elige.
- Qué hacer con un Report de era CLOSED que llega en HALF-OPEN respecto al crédito: el spec dice que no puede cerrar/reabrir ni cambiar `n`; no dice si ese éxito/fallo aplica aritmética CLOSED.
- Radio de un cambio de firma: en este árbol solo tests y benches de `backendbackoff/` llaman `Gate.Allow` / `Gate.Report`; callers fuera del repo no se midieron.
- Implicaciones Yaegi del tipo de identidad elegido.
- Comportamiento a escala más allá del repro (latencia mayor que el cooldown, miles de inflight).

## Tensions

- La spec viva `std_go_backendbackoff_cooldown` ya exige que el cooldown use solo el resultado de la sonda; el código toma el primer `Report` en HALF-OPEN.
- El informe scale-hunt §4 propone un permiso de generación y tratar el resto como era CLOSED ignorada; el spec del ticket no fija esa forma (explore no elige).
- El spec cita master `1f5ec454`; dest HEAD es `35aaa343c5185f8d5315e8715c3ceb05caed247a`. `backendbackoff/allow.go` sigue coincidiendo.
- Tres tickets `backendbackoff-*` tocan el mismo paquete.
