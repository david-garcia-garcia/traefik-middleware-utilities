# Requirement
IssueKey: 2026-09-11-simpleredis-perf-08-unsafe

## Problem
perf-08 measured `unsafe` zero-copy for `string`/`[]byte` on SimpleRedis verbs. Compiled it saves a little; under Yaegi (the Traefik plugin path this library ships in) it is a net loss, and adopting it would require `useUnsafe` so Traefik can register restricted symbols — or refuse to load the plugin if the operator has not opted in. The ask is to **not** adopt that conversion: record the measured decision in spec and usage docs, and keep/extend the Yaegi unsafe-variant tests as guards so a later change cannot sneak `unsafe` into production code.

## Current (code)
- `simpleredis/simpleredis.go` imports stdlib only (`bufio`, `errors`, `io`, `net`, `os`, `strconv`, `strings`, `sync`, `time`). No `unsafe`.
- `simpleredis/simpleredis.go` verbs `Get`/`MGet`/`Set`/`Del`/`Incr`/`IncrBy`/`Expire`/`ExpireAt`/`Eval` (lines 88–164) convert names, scripts, and decimal args with `[]byte(...)`. `parseIntegerReply` (lines 167–179) uses `strconv.ParseInt(string(values[0]), 10, 64)`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — session source MUST NOT use `unsafe`, cgo, or type parameters; Traefik local plugin `useunsafe` MUST be false.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — Yaegi tests MUST use GOPATH + stdlib only and `useunsafe` false; Pester `/redis` and `/dragonfly` must prove Get/MGet/Del/Incr/IncrBy/Expire/ExpireAt/Eval; Eval Lua 5.1-safe with KEYS declared; EVALSHA MUST NOT be added.
- `knowledge/devdocs/std_go_simpleredis.md` — Yaegi tests copy sources with stdlib only (`useunsafe` false); Eval keys must be listed (Dragonfly); no `table.maxn`. Does not record the measured “do not adopt zero-copy unsafe” decision.
- `simpleredis/yaegi_test.go` — `TestYaegi_InitGetSetDel`, `TestYaegi_IncrAndEval`; interp uses `stdlib.Symbols` only (comment: no unsafe). `TestYaegiUnsafeVariants` **not found**.
- `simpleredis/interpretedcost_test.go` **not found** on DestBranch. Named benches `BenchmarkCompiledEncodeEval{Copy,Unsafe}`, `BenchmarkCompiledParseInt{Copy,Unsafe}`, `BenchmarkYaegiConvert{Copy,CopyCall,Unsafe}` **not found**.
- `simpleredis/bench_test.go` **not found** on DestBranch.
- `docker-compose.yml` — `simpleredisprobe` (and `reclaimprobe`) `settings.useunsafe=false`; Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`; routes `/redis` and `/dragonfly`.
- `e2e/simpleredisprobe/.traefik.yml` — no `useUnsafe` field (defaults false). Probe Eval uses KEYS-declared Lua 5.1-safe INCRBY+EXPIREAT (`e2e/simpleredisprobe/plugin.go`).
- `scripts/integration-tests.Tests.ps1` — Pester asserts every SimpleRedis verb on GET `/redis` and GET `/dragonfly`.
- `simpleredis/` has no `*_LIVE_REDIS` / `*_LIVE_DRAGONFLY` Go tests (**not found**); live proof of verbs on both engines is compose + Pester.

## Desired
- Durable spec and `knowledge/devdocs/std_go_simpleredis.md` decision: do not adopt `unsafe` zero-copy; do not add unsafe conversions in session source; do not set `useUnsafe` on the plugin manifest or compose. Keep today’s `[]byte(...)` / `string(...)` conversions.
- Keep/extend `TestYaegiUnsafeVariants` (and the finding’s named copy-vs-unsafe benches in `simpleredis/interpretedcost_test.go`) as guards so a future Yaegi bump or a sneaky production `unsafe` import cannot land unnoticed.
- Existing verbs stay proven on **both** Redis and Dragonfly (compose + Pester `/redis` `/dragonfly`). Lua 5.1-safe. Dragonfly KEYS required. Do not drop those proofs.

## Affected
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` and/or `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (decision + guard scenarios)
- `knowledge/devdocs/std_go_simpleredis.md`
- `simpleredis/` tests (land/extend `TestYaegiUnsafeVariants` and the named benches; production `simpleredis.go` unchanged)
- `knowledge/research/ext_traefik_plugins_useunsafe/`, `knowledge/research/ext_traefik_plugins_yaegi-unsafe/` (third-party facts for later phases)

## Out of scope
- Adopting `unsafe` conversions in `simpleredis.go`.
- Setting `useUnsafe: true` on `.traefik.yml` or compose.
- The finding’s “If you adopt it anyway” playbook (legacy forms, inline helpers, GC-stress of unsafe tricks, dual build path).
- EVALSHA / SCRIPT LOAD (perf-05).
- Single-write encoding (perf-06) and `readLine`/`ReadSlice` (perf-07).
- go-redis, pool caps, I/O timeouts, pipelining, new RESP verbs.

## Unknowns
- Guard shape: log-only capability matrix vs tests that fail if `simpleredis.go` imports `unsafe` or if probe/compose `useUnsafe` becomes true.
- Whether compiled unsafe helpers in `*_test.go` (needed by the named benches) are the accepted way to keep the measurement without violating the production ban.
- `interpretedcost_test.go` on the caller’s uncommitted tree also holds perf-06 encode benches; whether those ride along when landing the perf-08 guards is for explore (this ticket does not take perf-06).

## Tensions
- Finding section “If you adopt it anyway” vs caller: the proposed action is **not** to adopt unsafe zero-copy — follow the caller.
- Finding and index point at `TestYaegiUnsafeVariants` / `simpleredis/interpretedcost_test.go` as if they exist; DestBranch does not have those files (`not found`). Landing them is part of Desired, not current.
- Spec already MUST NOT use `unsafe` and MUST keep `useunsafe` false; this ticket still wants the **measured** decision in spec/devdocs plus Yaegi-variant guards — reinforce, do not weaken or re-open adoption.
