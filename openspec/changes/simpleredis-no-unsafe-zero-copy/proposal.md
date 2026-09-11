## Why

perf-08 measured `unsafe` zero-copy between `string` and `[]byte` on SimpleRedis verbs: compiled it saves a little; under Yaegi (the Traefik plugin path) it is a net loss and would force `useUnsafe` so Traefik can register restricted symbols — or refuse to load the plugin. Dest already has no `unsafe` import and `useunsafe=false`, but spec/usage do not record the measured no-adopt decision, and Dest has no Yaegi-variant or production-sneak guards.

## What Changes

- Record in spec and usage that session source keeps `[]byte(...)` / `string(...)` conversions and MUST NOT add unsafe zero-copy helpers. Do not change `simpleredis.go`. Do not set `useUnsafe` on the plugin manifest or compose.
- Land `TestYaegiUnsafeVariants` as an asserting CI guard (fail when a matrix cell disagrees) plus the named copy-vs-unsafe benches in `simpleredis/interpretedcost_test.go`. Compiled `import "unsafe"` / `"C"` scan of non-test session files and a probe/compose `useUnsafe` scan so a sneak cannot land unnoticed.
- Existing verbs stay proven on Redis and Dragonfly (compose + Pester `/redis` `/dragonfly`). Lua 5.1-safe. Dragonfly KEYS required. No EVALSHA.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: session source SHALL keep `[]byte`/`string` conversions; SHALL NOT add unsafe zero-copy helpers; plugin `useunsafe` stays false.
- `std_go_simpleredis_resp-commands`: add the asserting Yaegi conversion matrix, named copy-vs-unsafe benches, and compiled session-source / probe-manifest `useUnsafe` scan. Do not weaken existing stdlib-only Yaegi Init/Get/Set/Del/Incr/Eval or Redis/Dragonfly Pester proofs.

## Impact

- `simpleredis/` tests only (`interpretedcost_test.go`, import/manifest scan in `simpleredis_test.go`, widen `writeGopathFile` to `testing.TB`). Production `simpleredis.go` unchanged.
- `knowledge/devdocs/std_go_simpleredis.md` records the measured decision and how the guards fail.
- Main specs `openspec/specs/std_go_simpleredis_tcp-session/spec.md` and `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Compose, probe Eval, Pester `/redis` `/dragonfly`, and reclaim routes stay as they are.
