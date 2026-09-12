# Explore
IssueKey: 2026-09-11-simpleredis-perf-08-unsafe

Verdict: in progress

## Concepts

perf-08 measured `unsafe` zero-copy for `string`/`[]byte` on SimpleRedis verbs and recommended **not** to adopt it. This ticket records that decision and lands guards. It does not change `simpleredis/simpleredis.go`. Dest already has no `unsafe` import and already `useunsafe=false`. The gap is proof: the finding's `TestYaegiUnsafeVariants` / `interpretedcost_test.go` are **not found** on DestBranch; usage docs did not record the measured loss; spec already MUST NOT use `unsafe` but does not name the measured reason or the bump/sneak guards.

```
  production path (do not change)          guard path (this change)
  ────────────────────────────────         ─────────────────────────
  simpleredis.go                           interpretedcost_test.go
    []byte(name) / string(values[0])        TestYaegiUnsafeVariants (assert matrix)
    no import "unsafe"                       Benchmark*Copy vs *Unsafe (reproduce ns)
                                           simpleredis_test.go
                                             session source import scan
                                             probe/compose useUnsafe scan

  Traefik Yaegi load (already false)
  .traefik.yml  useUnsafe  ──AND──  compose settings.useunsafe
        │                              │
        └──────── both true ──────────┘ → i.Use(unsafe.Symbols)
                  else                 → stdlib.Symbols only
                  manifest true +
                  settings false         → refuse to load plugin
```

Pinned third-party facts (prepare research, consume-enough — no new write):

- Dual AND gate: `knowledge/research/ext_traefik_plugins_useunsafe/` (traefik@v3.7.11 `newInterpreter`).
- Yaegi v0.16.1 `stdlib/unsafe` exports `Pointer`, `Add`, `Sizeof`, `Alignof`, `Offsetof` only. `Slice` / `String` / `StringData` / `SliceData` absent even with those symbols registered (`knowledge/research/ext_traefik_plugins_yaegi-unsafe/`).
- Dragonfly EVAL still requires KEYS; Lua 5.1-safe (no `table.maxn`) — `knowledge/research/ext_dragonfly_eval/`. Existing probe script already matches. Do not touch it.

Finding matrix (must become a **failing** test, not `t.Logf`):

| Conversion | stdlib only | stdlib+unsafe | +unrestricted |
|---|---|---|---|
| go-redis v9 `unsafe.Slice` / `unsafe.String` | no | no | no |
| `*(*string)(unsafe.Pointer(&b))` | no | yes | yes |
| struct-header `string`→`[]byte` | no | yes | yes |
| `reflect.StringHeader` | no | yes | yes |

Caller's uncommitted tree already has `simpleredis/interpretedcost_test.go` (log-only matrix + named benches + **perf-06** encode benches + `BenchmarkYaegiGet`) and `simpleredis/bench_test.go` (`tokenBucketScript` + hot-path benches). Dest has neither. `writeGopathFile` on dest takes `*testing.T`; `startFakeRedis` takes `*testing.T`. CI is `go test -timeout 2m -count=1 -v ./...` (no `-bench`).

Live verb proof on dest is compose + Pester `GET /redis` and `GET /dragonfly` (every verb header). No `*_LIVE_REDIS` / `*_LIVE_DRAGONFLY` Go tests. Probe Eval is KEYS-declared Lua 5.1-safe INCRBY+EXPIREAT.

Reclaim already has `TestTable_StdlibImports` (`parser.ParseFile` `ImportsOnly`, fail if import path contains `.`). That check **does not catch** `import "unsafe"` (`unsafe` has no dot). SimpleRedis has no import-scan test today.

No identity reconstruction (client address, user, tenant, request Host). `Init` host is the Redis server address from caller config.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` now records the measured no-adopt decision (explore produce). Spec deltas wait for propose.

## Decisions

**Do not adopt unsafe zero-copy in session source.** Keep `[]byte(...)` / `string(...)` on dest `simpleredis.go:88-164` and `parseIntegerReply`. Do not add `stringToBytesUnsafe` / `bytesToStringUnsafe` to production. Out of scope forbids the finding's "If you adopt it anyway" playbook (legacy forms inlined at use sites, `useUnsafe: true`, GC-stress, dual build).

**Record the measured reason in existing spec leaves, do not weaken them.** `std_go_simpleredis_tcp-session` already MUST NOT use `unsafe` / cgo / type parameters and MUST keep plugin `useunsafe` false. Propose adds: session source SHALL keep `[]byte`/`string` conversions; SHALL NOT add unsafe zero-copy helpers. `std_go_simpleredis_resp-commands` already requires Yaegi Init/Get/Set/Del/Incr/Eval with GOPATH + stdlib only. Propose adds a **separate** requirement for the capability-matrix test and named benches (those tests MAY register `yaegiunsafe.Symbols` and MAY `import "unsafe"` in `*_test.go`). Do not change the existing Init/Get/… Yaegi scenarios to register unsafe.

**Land `simpleredis/interpretedcost_test.go` with only the perf-08 named tests.** `TestYaegiUnsafeVariants` plus `BenchmarkCompiledEncodeEval{Copy,Unsafe}`, `BenchmarkCompiledParseInt{Copy,Unsafe}`, `BenchmarkYaegiConvert{Copy,CopyCall,Unsafe}`. Define the ~470-byte Eval fixture const in that file (dest has no `tokenBucketScript`; do not land `bench_test.go`). Do not copy `encodeprobeSrc`, `BenchmarkYaegiEncode{Bufio,SingleWrite}`, or `BenchmarkYaegiGet` (perf-06 / general interpreted Get; this ticket does not take perf-06).

**Guards fail CI; benches reproduce numbers.** `TestYaegiUnsafeVariants` asserts the table above (`t.Fatal` when a cell disagrees). A Yaegi bump that adds `Slice` or that starts exporting unsafe under `stdlib.Symbols` fails the test so a human sees it — do not re-open adoption in this change. Compiled `go test ./...` does not run benches; they stay for `go test -run XXX -bench`. Do not fail CI on ns/op.

**Production sneak is a compiled import/manifest scan, not the matrix.** Follow reclaim's `parser.ParseFile` `ImportsOnly` pattern in `simpleredis_test.go`, but fail on import path `unsafe` or `"C"` (and non-stdlib dotted paths). Scan `e2e/simpleredisprobe/.traefik.yml` and the compose `simpleredisprobe.settings.useunsafe` line: absent or false passes; `true` fails. Do not require an explicit `useUnsafe: false` on `.traefik.yml` (omit still defaults false). Do not scan `reclaimprobe`.

**Compiled unsafe helpers live only in `*_test.go`.** `writeGopathSimpleredis` already skips `_test.go`, so GOPATH copies of session source stay stdlib-only. `BenchmarkYaegiConvert*` and the with-unsafe matrix modes register `yaegiunsafe.Symbols` against **probe source strings**, not against `simpleredis.go`.

**Widen `writeGopathFile` to `testing.TB`.** Two call sites in `simpleredis/yaegi_test.go` (searched `simpleredis/**` for `writeGopathFile`); both pass `*testing.T`, which implements `testing.TB`, so they keep compiling. Leave `writeGopathSimpleredis` and `startFakeRedis` as `*testing.T` (this change does not land `BenchmarkYaegiGet` / `bench_test.go`).

**Do not touch Redis/Dragonfly live proofs.** Keep compose `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, Pester `/redis` `/dragonfly`, probe Eval KEYS + Lua 5.1-safe. Do not add `*_LIVE_*` Go tests. Do not add EVALSHA.

**No new research folder.** `ext_traefik_plugins_useunsafe` and `ext_traefik_plugins_yaegi-unsafe` answer the vendor facts. Redis/Dragonfly verb behavior is unchanged.

## Open questions

- Q: Guard shape — log-only capability matrix vs tests that fail if session source imports `unsafe` or probe/compose `useUnsafe` becomes true?
  Rank: additive asked — new tests this change creates; Desired names guards so a future Yaegi bump or sneaky production `unsafe` cannot land unnoticed; Unknowns line 1
  Decision: assumed — asserting matrix in `TestYaegiUnsafeVariants` (fail on cell mismatch). Separate compiled scan fails if non-test files in `simpleredis/` import `unsafe` or `"C"`, or if simpleredisprobe manifest/compose `useUnsafe` is true. Log-only is not a CI guard (`go test ./...` would stay green).
  By: explore

- Q: Are compiled unsafe helpers in `*_test.go` the accepted way to keep the measurement without violating the production ban?
  Rank: additive asked — Desired names the copy-vs-unsafe benches; Unknowns line 2; tcp-session MUST NOT `unsafe` names session source, not `_test.go`
  Decision: assumed — yes. `import "unsafe"` and `yaegiunsafe.Symbols` only in `*_test.go` and in interpreted probe source strings. Do not put helpers in `simpleredis.go`. GOPATH copy still skips `_test.go`. Existing `TestYaegi_InitGetSetDel` / `TestYaegi_IncrAndEval` stay stdlib-only.
  By: explore

- Q: Do perf-06 encode benches in the caller's uncommitted `interpretedcost_test.go` ride along when landing the perf-08 guards?
  Rank: additive asked — Out of scope names single-write encoding (perf-06); Unknowns line 3; Desired names the finding's listed benches only
  Decision: assumed — no. Land only the perf-08 named tests/benches in `simpleredis/interpretedcost_test.go`. Do not land `encodeprobeSrc`, `BenchmarkYaegiEncode*`, `BenchmarkYaegiGet`, or `bench_test.go`. Copy the Eval-size fixture const into `interpretedcost_test.go`.
  By: explore

- Q: Which spec leaf records the measured no-adopt decision vs the Yaegi-variant guards?
  Rank: additive asked — Desired names durable spec + usage; Affected names both `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands`
  Decision: assumed — tcp-session: keep `[]byte`/`string`; MUST NOT add unsafe zero-copy in session source; plugin `useunsafe` stays false. resp-commands: add scenarios for the asserting matrix, named benches, and the source/manifest scan. Do not weaken the existing stdlib-only Yaegi Init/Get/… requirement. Usage Gotchas already record the measured loss (explore produce).
  By: explore

- Q: Does landing the named Yaegi convert benches require changing `writeGopathFile`?
  Rank: additive incidental — unexported test helper; 2 call sites in `simpleredis/yaegi_test.go` (searched `simpleredis/**` for `writeGopathFile`); they keep working because `*testing.T` implements `testing.TB`; no criterion names the helper
  Decision: assumed — widen `writeGopathFile` to `testing.TB` so `BenchmarkYaegiConvert*` can write GOPATH files. Do not change `writeGopathSimpleredis` or `startFakeRedis`.
  By: explore

- Q: Must new guards run against live Redis and Dragonfly, or do existing Pester routes stay the verb proof?
  Rank: additive asked — Desired: existing verbs stay proven on both engines (compose + Pester `/redis` `/dragonfly`); Current: no `*_LIVE_*` Go tests
  Decision: resolved — keep compose + Pester as the live proof. Do not add live Go tests. Do not edit probe Eval, Pester headers, or engine pins. New guards are in-process (`go test ./simpleredis/...`).
  By: explore

- Q: Should `.traefik.yml` grow an explicit `useUnsafe: false` so the invariant is visible?
  Rank: additive incidental — Desired says do not set `useUnsafe` on the manifest; Out of scope names setting `useUnsafe: true`; dest omits the field (defaults false)
  Decision: assumed — leave the field omitted. Guard test passes on absent or false; fails on true. Compose keeps `--experimental.localplugins.simpleredisprobe.settings.useunsafe=false`.
  By: explore

- Q: Are the named benches CI-failing guards or reproduction-only?
  Rank: additive asked — Desired names those benches as guards; dest CI is `go test` without `-bench` (`.github/workflows/ci.yml`)
  Decision: assumed — benches reproduce the finding's ns/op (`go test -run XXX -bench`). CI guard is the asserting `TestYaegiUnsafeVariants` plus the import/useUnsafe scan. Do not fail the suite on allocation or ns/op drift.
  By: explore
