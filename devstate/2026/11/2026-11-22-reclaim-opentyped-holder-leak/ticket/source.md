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
