# backendbackoff: half of the capped cooldowns land exactly on MaxCooldown

## Problem

`cooldownDuration` jitters as `wait × (1 + J × (2u − 1))`, u ∈ [0,1), then clamps to `MaxCooldown`. Once `wait` is already at the cap, every sample with u > 0.5 overshoots and is clamped to exactly the max: a 50% point mass on one value (measured 50.0% over 100,000 samples). The code comment states the goal "so replicas that trip together do not probe in lockstep"; at the cap half of them do. During a long shared outage, R replicas × K keys produce ~R·K/2 simultaneous probes per cycle against a backend that is trying to recover.

## Current (code, master 1f5ec454)

- `backendbackoff/gate.go:156-182` `cooldownDuration` (clamp at `:178`).
- Spec `openspec/specs/std_go_backendbackoff_cooldown/spec.md` requirement "Cooldown opens and probes" fixes the formula `cooldown * (1 + Jitter * (2u - 1))` capped at MaxCooldown, so the spec text must change too.

## Evidence

Repro test: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\backendbackoff\zzz_scale_hunt_test.go` — `TestScale_JitterPointMassAtMax`. Ignore the other tests in that file (other tickets).

## Desired

Cooldowns never exceed MaxCooldown and no single value carries a point mass at the cap; jittered probes stay spread when the nominal cooldown is at the cap. Jitter 0 keeps disabling jitter exactly. Spec and tests updated.

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

- `backendbackoff/gate.go:156-182` `cooldownDuration`: duplica `BaseCooldown` hasta `MaxCooldown`, luego si `Jitter > 0` aplica `wait * (1 + Jitter*(2u-1))`.
- `backendbackoff/gate.go:178-179` clamp: si `jittered > MaxCooldown` devuelve exactamente `MaxCooldown`.
- `backendbackoff/gate.go:172` comentario: "Spread wait by ±Jitter so replicas that trip together do not probe in lockstep."
- `backendbackoff/gate.go:169-170` `Jitter <= 0` devuelve `wait` sin jitter.
- `backendbackoff/gate.go:175-176` clamp inferior: `jittered < 0` pasa a 0.
- `backendbackoff/gate.go:33` y `:109-110`: Config parcial con Jitter 0 desactiva jitter; Config todo-cero aplica `defaultJitter` 0.10.
- `backendbackoff/gate.go:16,124-125` `Jitter` debe estar en `[0, 1)`.
- `backendbackoff/allow.go:72` sonda fallida: `openUntil = now.Add(g.cooldownDuration(entry.n))` tras `n++`.
- `backendbackoff/allow.go:86` trip desde CLOSED: `openUntil = now.Add(g.cooldownDuration(entry.n))`.
- `openspec/specs/std_go_backendbackoff_cooldown/spec.md:7-8` requisito "Cooldown opens and probes": fórmula `cooldown * (1 + Jitter * (2u - 1))` capped at `MaxCooldown`.
- `openspec/specs/std_go_backendbackoff_allow/spec.md:63` `Jitter` 0 SHALL disable jitter.
- `backendbackoff/zzz_gate_test.go:402-410` `newTestGate` fija `Jitter: 0`; los tests de cooldown congelan la espera.
- `backendbackoff/zzz_gate_test.go` — not found un test que mida la distribución del jitter en el cap.
- `backendbackoff/zzz_scale_hunt_test.go` / `TestScale_JitterPointMassAtMax` — not found en este worktree (vive en el checkout scale-hunt).
- `knowledge/devdocs/std_go_backendbackoff.md` describe el cooldown exponencial; no nombra la masa puntual en `MaxCooldown`.

## Out of scope

- Recorrido O(n) del mapa al crear clave (`backendbackoff/allow.go` `loadEntry`) — otro ticket del lote.
- `Report` sin identidad / sonda HALF-OPEN decidida por una respuesta vieja — otro ticket del lote.
- `iplookup`, `windowcounter`, `reclaim`, `tokenbucket`, `simpleredis`.
- Archivos helper compartidos entre los siete PRs.
- Cambiar las restricciones Yaegi.
- Elegir el algoritmo final en explore (la nota de proceso reserva propose al tech lead).
- La deuda de capa compartida entre réplicas (`knowledge/debt/2026-09-12-backendbackoff-shared-layer.md`).
- Cambiar `FailureRatio`, `TripFailures`, `TTL`, el lease de sonda, o el reset de `n` tras un tramo CLOSED de `MaxCooldown`.
- El jitter de reintentos de `simpleredis`.

## Unknowns

- Qué fórmula de muestreo cumple Desired. El informe de caza sugiere uniforme en `[Max*(1-J), Max]` cuando el nominal ya está en el cap; Desired no fija esa fórmula.
- Si el jitter por debajo del cap también produce recorte (overshoot simétrico cuando `wait` está cerca de `MaxCooldown`, no solo cuando `wait == MaxCooldown`).
- Si el recorte `float64` → `time.Duration` crea masas puntuales extra.
- Si `rng.Float64()` es exactamente `[0, 1)` como dice el spec.
- Radio de los tests de dest que asumen la fórmula actual (casi todos usan `Jitter: 0`).
- Hunks de merge contra los otros dos PRs `backendbackoff-*` del lote.
- La masa puntual del 50,0 % no se ha vuelto a medir en este worktree (prepare no ejecuta el repro).

## Tensions

- La letra del spec vivo (`openspec/specs/std_go_backendbackoff_cooldown/spec.md:8`, "plus jitter, capped at MaxCooldown") coincide con el clamp de `backendbackoff/gate.go:178-179`. Desired exige que esa fórmula cambie para que no haya masa puntual en el cap. El comentario en `gate.go:172` ya contradice la letra del spec cuando `wait` está en el máximo.
- El spec del caller cita `master 1f5ec454`. Dest HEAD es `35aaa343`. El clamp de `cooldownDuration` sigue en las mismas líneas; dest no está stale para este subsistema.
- La "Dirección de arreglo" del informe de caza (muestreo uniforme en `[Max*(1-J), Max]`) es evidencia, no un diseño vinculante. Explore lista opciones y no elige; propose lo escribe el tech lead.
- Tres tickets `backendbackoff-*` del lote editan el mismo paquete; los hunks de este ticket deben quedarse en `cooldownDuration`, el spec de cooldown, y tests propios.
