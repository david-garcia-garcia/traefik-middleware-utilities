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
