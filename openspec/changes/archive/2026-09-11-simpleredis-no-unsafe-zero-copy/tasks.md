## 1. Yaegi matrix and benches

- [x] 1.1 Widen `writeGopathFile` in `simpleredis/yaegi_test.go` to `testing.TB`. Leave `writeGopathSimpleredis` and `startFakeRedis` as `*testing.T`
- [x] 1.2 Add `simpleredis/interpretedcost_test.go` with asserting `TestYaegiUnsafeVariants` (fail on cell mismatch), the ~470-byte Eval fixture const, `stringToBytesUnsafe`/`bytesToStringUnsafe` in that file only, and named benches `BenchmarkCompiledEncodeEval{Copy,Unsafe}`, `BenchmarkCompiledParseInt{Copy,Unsafe}`, `BenchmarkYaegiConvert{Copy,CopyCall,Unsafe}`. Do not land `encodeprobeSrc`, `BenchmarkYaegiEncode*`, `BenchmarkYaegiGet`, or `bench_test.go`. Do not change `simpleredis.go`
- [x] 1.3 Run `go test ./simpleredis/...` until `TestYaegiUnsafeVariants` and existing `TestYaegi_InitGetSetDel` / `TestYaegi_IncrAndEval` pass

## 2. Production sneak scan

- [x] 2.1 Add a compiled import scan of non-test `simpleredis/*.go` (`parser.ParseFile` `ImportsOnly`): fail on `unsafe`, `"C"`, or a dotted path. `_test.go` MAY import `unsafe`
- [x] 2.2 Add a compiled scan of `e2e/simpleredisprobe/.traefik.yml` and compose `simpleredisprobe.settings.useunsafe`: absent or false passes; true fails. Do not add `useUnsafe` to the manifest. Do not scan `reclaimprobe`. Do not edit probe Eval, Pester, or engine pins
- [x] 2.3 Run `go test ./simpleredis/...` until the scans pass

## 3. Specs

- [x] 3.1 Confirm the change deltas `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` match the landed tests
- [x] 3.2 Run `openspec validate --change simpleredis-no-unsafe-zero-copy --strict`
