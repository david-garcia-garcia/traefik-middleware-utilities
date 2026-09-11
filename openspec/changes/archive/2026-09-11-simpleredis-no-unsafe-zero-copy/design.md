## Context

Dest session source already converts with `[]byte(...)` / `string(...)` and does not import `unsafe`. Plugin compose keeps `settings.useunsafe=false`; `.traefik.yml` omits `useUnsafe` (defaults false). See proposal.md for why. Research: `knowledge/research/ext_traefik_plugins_useunsafe/`, `ext_traefik_plugins_yaegi-unsafe/`. Proceed policies: `devstate/explore.md`. Existing Yaegi Init/Get/Set/Del/Incr/Eval stay stdlib-only. Live verb proof stays compose + Pester `/redis` `/dragonfly`.

## Goals / Non-Goals

**Goals:**
- Asserting Yaegi conversion matrix plus named copy-vs-unsafe benches in `simpleredis/interpretedcost_test.go`.
- Compiled import scan of non-test session files and probe/compose `useUnsafe` scan.
- Widen `writeGopathFile` to `testing.TB` so interpreted convert benches can write GOPATH files.

**Non-Goals:**
- Changing `simpleredis.go`, probe Eval, compose engines, Pester headers, or reclaimprobe.
- Landing `bench_test.go`, `BenchmarkYaegiEncode*`, `BenchmarkYaegiGet`, or `encodeprobeSrc` (perf-06).
- Changing `writeGopathSimpleredis` or `startFakeRedis`.
- Adopting unsafe, setting `useUnsafe: true`, or adding EVALSHA.

## Decisions

1. **Assert the matrix; do not log-only.** `TestYaegiUnsafeVariants` fails (`t.Fatal`) when a cell disagrees with the measured table: go-redis v9 `unsafe.Slice`/`unsafe.String` unsupported in every mode; legacy pointer-cast, struct-header, and `reflect.StringHeader` unsupported under `stdlib.Symbols` and supported when `yaegiunsafe.Symbols` are registered (with or without unrestricted). Alternative: `t.Logf` like the caller's uncommitted file — rejected; `go test ./...` would stay green on a Yaegi bump.

2. **Unsafe helpers only in `_test.go` and interpreted probe source strings.** Compiled benches call unexported `stringToBytesUnsafe` / `bytesToStringUnsafe` in `interpretedcost_test.go`. Yaegi convert benches register `yaegiunsafe.Symbols` against `convertprobe` source, not against `simpleredis.go`. GOPATH copy still skips `_test.go`. Alternative: helpers in session source — out of scope.

3. **Eval-size fixture const lives in `interpretedcost_test.go`.** Dest has no `tokenBucketScript`. Copy the ~470-byte Lua const into that file so Eval-encode benches compile without landing `bench_test.go`. Alternative: share a production const — rejected; session source stays unchanged.

4. **Import scan follows reclaim's `parser.ParseFile` `ImportsOnly`, then also fails `unsafe` and `"C"`.** Reclaim's dotted-path check misses `unsafe` (no dot). Scan non-test `*.go` in `simpleredis/` only. `_test.go` MAY import `unsafe`. Alternative: string-search the file — rejected; parse imports.

5. **Manifest/compose scan: absent or false passes; true fails.** Do not add `useUnsafe: false` to `.traefik.yml`. Do not scan `reclaimprobe`. Compose keeps `--experimental.localplugins.simpleredisprobe.settings.useunsafe=false`. Alternative: require an explicit false on the manifest — rejected; Desired says do not set `useUnsafe`.

6. **Widen only `writeGopathFile` to `testing.TB`.** Two dest call sites in `yaegi_test.go` pass `*testing.T`. Leave `writeGopathSimpleredis` and `startFakeRedis` as `*testing.T`. Alternative: also widen those — not needed; this change does not land `BenchmarkYaegiGet`.

## Risks / Trade-offs

- [A future Yaegi bump that adds `Slice` fails CI] → Mitigation: that is the point; a human decides. This change does not re-open adoption.
- [Compiled benches import `unsafe` in the same package] → Mitigation: GOPATH copy skips `_test.go`; production scan fails if session source grows the import.
- [Compose `useunsafe=false` line is edited in a way the scan misses] → Mitigation: fail on a true assignment to `simpleredisprobe.settings.useunsafe`; keep the existing false line.
- [Pester Redis/Dragonfly proofs drift while this ticket only adds in-process tests] → Mitigation: do not edit those files; existing requirement stays.

## Migration Plan

Test-only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
