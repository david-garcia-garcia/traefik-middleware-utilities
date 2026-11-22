# windowcounter: buffered flush tick scans every window under the limiter mutex

## Problem

In buffered mode (`syncRate > 0`) every flush tick walks the whole `l.windows` map twice while holding `l.mu`: once to collect windows with `localDelta > 0`, once to prune expired windows. Both walks are O(K) in the number of live windows even when nothing is dirty. #105 batched the EVALs but left both walks. The map has no cap, and K ≈ distinct keys seen in the last 2-3 windows (a current window lives until `windowStart + 2*window`), so per-IP limiting under IP rotation drives K. The timer re-arms after the flush returns, so the mutex duty is about `scan / (scan + syncRate)`. Every Take/Peek of that Limiter, for any key, waits behind the scan. `Sleep` and `Close` pay the same scan.

## Current (code, master 1f5ec454)

- `windowcounter/limiter.go:494-530` `flushPendingLocked`: collect loop over `l.windows` and prune loop over `l.windows`, both under `l.mu`.
- `windowcounter/limiter.go:444-455` `flushAfterTick` re-arms after `flushPending` returns; `Sleep` (`:398`) and `Close` (`:414`) call `flushPending`.
- `windowcounter/limiter.go:131-158` `slidingAt`: `expireAt = windowStart + 2*windowSec`.

## Evidence (measured, idle tick = nothing to flush)

| Live windows | Compiled | Yaegi |
|---:|---:|---:|
| 10,000 | 0.2 ms | 1.2 ms |
| 50,000 | — | 8.6 ms |
| 200,000 | 11.3 ms | 55.7 ms |
| 1,000,000 | 65.6 ms | — |

At syncRate 20 ms and 200k windows under Yaegi the mutex is busy ~74% of the time (avg ~20 ms added to every Take/Peek, worst ~56 ms).

Repro tests: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\windowcounter\zzz_scale_hunt_test.go` — `TestScale_FlushTickScanCost` (compiled) and `TestScale_YaegiFlushTickScanCost` (Yaegi, white-box probe copied into the interpreted package). Ignore `TestScale_ConcurrentSleepClearsStopping` in that file (other ticket).

## Desired

An idle or mostly-idle flush tick costs O(dirty windows + windows that expire now), not O(all live windows), in both compiled and Yaegi, without changing admission results, flush semantics (spec `std_go_windowcounter_sync-flush`) or losing any pending delta. A regression test/benchmark proves that the mutex hold time of an idle tick does not grow with the number of live windows.

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

Comprobado en este worktree (dest `master` HEAD `35aaa343c5185f8d5315e8715c3ceb05caed247a`). El spec cita `1f5ec454`; las rutas siguen coincidiendo.

- `windowcounter/limiter.go:63` `windows map[string]*windowState` — sin tope en el tipo ni en `New` (`:97-107`).
- `windowcounter/limiter.go:131-158` `slidingAt` — `ttlSec := 2 * windowSec` (`:145`); `expireAt: windowStart + ttlSec` (`:151`).
- `windowcounter/limiter.go:171-183` `Take` — modo buffered llama `takeBuffered`.
- `windowcounter/limiter.go:187-199` `Peek` — modo buffered llama `peekBuffered`.
- `windowcounter/limiter.go:227-235` `takeBuffered` — `l.mu.Lock` / `Unlock` alrededor del incremento.
- `windowcounter/limiter.go:251-258` `peekBuffered` — `l.mu.Lock` / `Unlock` alrededor de la lectura.
- `windowcounter/limiter.go:397-401` `Sleep` — llama `flushPending` en `:399` (el spec cita `:398`, inicio del comentario).
- `windowcounter/limiter.go:414-424` `Close` — la función empieza en `:414`; llama `flushPending` en `:422`.
- `windowcounter/limiter.go:443-454` `flushAfterTick` — `flushPending` en `:445`; rearma el timer en `:452` después de volver.
- `windowcounter/limiter.go:477-482` `flushPending` — toma `l.mu` y llama `flushPendingLocked`.
- `windowcounter/limiter.go:494-530` `flushPendingLocked` — bucle de recogida sobre `l.windows` (`:498-507`) y bucle de poda sobre `l.windows` (`:520-524`), ambos con `l.mu` del caller.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — contrato vivo de flush buffered / exact.
- `windowcounter/zzz_scale_hunt_test.go` — not found (este worktree).
- `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\windowcounter\zzz_scale_hunt_test.go` — existe fuera de este árbol (`TestScale_FlushTickScanCost`, `TestScale_YaegiFlushTickScanCost`).

## Out of scope

- Los otros seis tickets del lote (incluido `TestScale_ConcurrentSleepClearsStopping` / Sleep concurrente).
- Cambiar resultados de admisión o la semántica de flush del spec `std_go_windowcounter_sync-flush`.
- Escribir la fase propose (la hace el tech lead).
- Ficheros helper compartidos entre PRs paralelos.
- Un tope de ventanas como política de producto distinta del coste O(sucias + que caducan ahora).
- Editar `backendbackoff` u otros paquetes.

## Unknowns

- Qué estructura (conjunto sucio, agrupación por `windowStart`, u otra) cumple el coste bajo Yaegi v0.16.1 sin perder deltas. Explore lista opciones; no elige el diseño.
- Tiempo de mutex de un tick idle re-medido sobre dest `35aaa343` (las cifras del spec son de `1f5ec454`).
- Radio de impacto de cambiar `flushPendingLocked` frente a Sleep/Close/Take/Peek y a los tests `zzz_*` ya en el paquete.
- Cómo afirmar el hold time del mutex en un test de regresión sin tautología y sin copiar un probe de caja blanca que choque con Yaegi.
- Si un tope de mapa hace falta para el coste pedido o es otro producto.

## Tensions

- El spec cita `master 1f5ec454`; dest HEAD en este worktree es `35aaa343c5185f8d5315e8715c3ceb05caed247a`. Las líneas de código del problema coinciden; las medidas de runtime no se han repetido aquí.
- El spec apunta `Close` (`:414`); la llamada a `flushPending` está en `:422`.
- `scale-bug-report.md` § 1 ya nombra conjunto sucio + agrupación por inicio de ventana + tope; la nota de proceso dice que explore no elige el diseño final.
- Los repros viven en el worktree `scale-hunt`, no en esta rama.
