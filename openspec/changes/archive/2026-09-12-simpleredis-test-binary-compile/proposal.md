## Why

`interpretedcost_test.go` calls `writeGopathFile` at four sites. If that helper is missing, `go test ./simpleredis/` fails to compile and no test in the package runs — including the Yaegi interpreter cases the existing spec already requires. Dest already has the helper; this change locks that pairing so a later commit cannot land the callers without the helper.

## What Changes

- Record that the SimpleRedis test package compiles as one binary: `writeGopathFile` lives in `yaegi_test.go` and `interpretedcost_test.go` is part of that package.
- Keep dest’s existing helper and tracked test files. Do not replace them with the caller’s untracked copies.
- No production `simpleredis.go` change unless dest compile fails (it does not).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: the test package SHALL compile; `writeGopathFile` SHALL satisfy the GOPATH writes in `interpretedcost_test.go` so interpreter tests actually run.

## Impact

- `simpleredis/yaegi_test.go` (`writeGopathFile`; `writeGopathClientprobe` already delegates)
- `simpleredis/interpretedcost_test.go` (four call sites)
- `simpleredis/bench_test.go` (tracked on dest; no helper call sites)
- CI `go test ./...` (already the durable compile guard)
- Out of scope: shared internal test-support package; `apm_modules/` gitignore; other `simpleredisfixes2` findings
