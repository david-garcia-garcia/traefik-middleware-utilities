## Why

Dest lint is a short `.golangci.yml` plus CI `version: latest`. Named results, `t.Helper()`, identity error compares the retry spec requires, and Yaegi `Eval` type asserts are unguarded. A future golangci-lint v2 release would also reject this config schema and turn unrelated PRs red.

## What Changes

- Enable the dest-zero linters plus `gocritic` (`unnamedResult` added, `checkExported: false`), `thelper`, `revive`, `dupword`, `prealloc`, `stylecheck`, `errname`, `forcetypeassert` (exclude `_test.go`), and `errorlint`.
- Fix every dest hit without changing runtime behavior. Name `borrow` / `dial` results by hand (`handshakeFailed`). Do not reorder `error` last.
- Document `//nolint:errorlint` on the ten identity `==` sites. Convert none to `errors.Is`.
- Do not enable `testpackage`. Omit `goimports` / `gofumpt` (Windows has no `diff`).
- Pin CI golangci-lint to `v1.63.4`. Exclude untracked `apm_modules` / `.agents` from typechecking.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_ci_test-suites`: the `lint` job SHALL pin golangci-lint to the validated v1.x (`v1.63.4`), SHALL run the enabled set in `.golangci.yml`, MUST NOT enable `testpackage`, and MUST NOT use golangci-lint v2.

## Impact

- `.golangci.yml`, `.github/workflows/ci.yml` (`version:` only).
- Mechanical Go fixes (named results, `t.Helper()`, `assignOp`, revive param order on unexported reclaim helpers, `//nolint:errorlint`, dupword/prealloc).
- `simpleredis/pool.go` `borrow` / `dial` result names (call-site compatible).
- `openspec/specs/std_go_ci_test-suites/spec.md` after archive.
- `knowledge/devdocs/std_go_test-suites.md` (pin, rejected `testpackage`, omitted goimports/gofumpt, `checkExported` trap).
- No runtime behavior change. Merge after in-flight SimpleRedis branches that rewrite `commands_exec.go`, `pool.go`, `resp.go`.
