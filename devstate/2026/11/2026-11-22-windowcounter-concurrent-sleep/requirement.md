# windowcounter: a second concurrent Sleep clears `stopping` while the first is still waiting

## Problem

`stopFlushAndWait`: Sleep #1 sets `stopping=true`, detaches the timer and waits on `flushBusy` for the in-flight tick. A concurrent Sleep #2 finds `flushTimer == nil`, sets `stopping=false` and returns at once. Consequences: Sleep #2 returns before the in-flight flush finished (caller believes it is flushed); a `Wake` in that gap starts a second timer chain, and Sleep #1's `flushBusy.Wait()` then waits for a chain that re-arms forever, so Sleep #1 can hang (through reclaim: that key stays busy and later Opens block). Trigger: two concurrent Sleep on one Limiter (several owners sharing a Limiter). Low severity, confirmed white-box.

## Current (code, master 1f5ec454)

- `windowcounter/limiter.go:457-474` `stopFlushAndWait`; `:398-412` `Sleep` / `Wake`; `:444-455` `flushAfterTick`.
- Earlier fix of the Sleep/Wake race: `windowcounter/BUGS.md` #3 history (`zzz_repro_sleep_wake_race_test.go`).

## Evidence

Repro test: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\windowcounter\zzz_scale_hunt_test.go` — `TestScale_ConcurrentSleepClearsStopping` (white-box simulation of Sleep #1 mid-wait). Ignore the other tests in that file (other ticket). A black-box test with a blocked flush EVAL and two real concurrent Sleeps plus a Wake is preferred for the PR.

## Desired

Concurrent Sleep calls are safe: none returns before the in-flight flush completes, a Wake during any pending stop cannot start a second chain, every Sleep returns, and at most one timer chain exists afterwards. Yaegi-safe (no interpreted select; AfterFunc model kept).

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

Contra dest HEAD `35aaa343c5185f8d5315e8715c3ceb05caed247a` (`origin/master` en este worktree), no contra el SHA del spec.

- `windowcounter/limiter.go:457-474` `stopFlushAndWait`: pone `stopping=true`, copia y anula `flushTimer`. Si el timer era `nil`, pone `stopping=false` y vuelve sin `flushBusy.Wait()`.
- `windowcounter/limiter.go:398-401` `Sleep`: `flushPending` y luego `stopFlushAndWait`.
- `windowcounter/limiter.go:404-411` `Wake`: no arranca si `closed`, `syncRate == 0`, `flushTimer != nil` o `stopping`; si no, `startFlushLocked`.
- `windowcounter/limiter.go:444-454` `flushAfterTick`: si `closed`, `stopping` o `flushTimer == nil`, hace `flushBusy.Done()` y no rearma; si no, arma otro `AfterFunc`.
- `windowcounter/limiter.go:65` campo `stopping`; `:67` `flushBusy`; `:414-424` `Close` también llama `stopFlushAndWait`.
- `windowcounter/zzz_repro_sleep_wake_race_test.go` — un `Sleep` frente a `Wake` concurrentes; no cubre dos `Sleep`.
- `windowcounter/zzz_fake_redis_test.go` `startTestFakeRedis`; `windowcounter/zzz_limiter_test.go` `newSimpleRedisForTest`.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` requisito Sleep/Wake/Close: un `Sleep` frente a `Wake` concurrente; no hay escenario de dos `Sleep`.
- `knowledge/devdocs/std_go_windowcounter.md` — mismo contrato de un `Sleep` frente a `Wake`.
- `windowcounter/BUGS.md` — not found
- `windowcounter/zzz_scale_hunt_test.go` `TestScale_ConcurrentSleepClearsStopping` — not found (vive en el checkout scale-hunt)

## Out of scope

- Los demás tests de `zzz_scale_hunt_test.go` (otro ticket: coste del barrido de flush).
- Los seis tickets hermanos de este lote; no tocar `backendbackoff`, `iplookup` ni el producto de `reclaim`.
- Pedir un arreglo de `Close` concurrente con `Sleep` (comparten `stopFlushAndWait`; el spec solo pide dos `Sleep`).
- Elegir el mecanismo (contador de paradas, generación, serializar la parada). Explore lista opciones; no es un requisito extra.
- Ficheros helper compartidos entre los siete PRs; `select` interpretado; cambiar el modelo `AfterFunc`.

## Unknowns

- Si el cuelgue de Sleep #1 se reproduce en negro (EVAL de flush bloqueado + dos `Sleep` reales + `Wake`) sobre este dest. No medido aquí.
- Qué dueño de la parada (generación, contador, serializar) cumple el efecto pedido. Explore lo lista; no lo decide.
- Si `Close` concurrente con `Sleep` cae por el mismo agujero en runtime.
- Cómo serializa reclaim las transiciones de cada clave en dest (el informe dice que el trigger por reclaim es poco probable).
- Coste y comportamiento Yaegi del arreglo; explore los mide.

## Tensions

- El spec cita master `1f5ec454`; dest HEAD es `35aaa343c5185f8d5315e8715c3ceb05caed247a`. Las líneas de `stopFlushAndWait` / `Sleep` / `Wake` / `flushAfterTick` siguen coincidiendo.
- El spec apunta a `windowcounter/BUGS.md` #3; ese fichero no está en dest. El test de la carrera Sleep/Wake sí: `windowcounter/zzz_repro_sleep_wake_race_test.go`.
- El spec vivo y el usage doc cubren un `Sleep` frente a `Wake`, no dos `Sleep` concurrentes. Ese hueco es el ticket.
- El informe §7 sugiere que solo quien inició la parada borre `stopping`, o serializarla. El Desired del spec es el efecto de producto, no ese mecanismo. Explore no elige diseño; propose lo escribe el tech lead.
