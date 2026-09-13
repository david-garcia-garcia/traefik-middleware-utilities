## 1. Config

- [x] 1.1 Rewrite `.golangci.yml`: keep dest ten, add the 17 zeros, `gocritic` (`enabled-checks: [unnamedResult]`, `checkExported: false` plus comment that `true` is exported-only), `thelper`, `revive`, `dupword`, `prealloc`, `stylecheck`, `errname`, `forcetypeassert`, `errorlint`. `issues.exclude-dirs`: `apm_modules`, `.agents`. `issues.exclude-rules`: `forcetypeassert` on `_test.go`. `max-issues-per-linter: 0`, `max-same-issues: 0`. Do not enable `testpackage`, `goimports`, or `gofumpt`.
- [x] 1.2 Pin `.github/workflows/ci.yml` job `lint` `version:` to `v1.63.4`

## 2. Named results and gocritic

- [x] 2.1 Name `borrow` and `dial` results `(conn *pooledConn, err error, handshakeFailed bool)` in `simpleredis/pool.go`. Keep explicit returns. `//nolint:revive` on `error-return` with the deferred-reorder reason. Do not reorder.
- [x] 2.2 Name every dest `unnamedResult` site (17: `simpleredis` `do`/`retryLimits`/test helpers, `windowcounter` Take/Peek/…, `tokenbucket`/`windowcounter` fake Redis). Explicit `return`. No naked returns.
- [x] 2.3 Apply `assignOp` in `tokenbucket/clock.go` (`+=`, `--`, `++`)

## 3. Other dest hits

- [x] 3.1 Add `t.Helper()` / `b.Helper()` and rename `testing.TB` params to `tb` at the 15 `thelper` sites
- [x] 3.2 Fix revive: `entry.credit--` in `backendbackoff/allow.go`; move `context.Context` to first param on `reclaim` `dropWhenDone`/`watch` and the two test helpers; unused `t` → `_`
- [x] 3.3 `dupword` in `tokenbucket/lua.go` (blank line between consecutive Lua `end`); `prealloc` in `reclaim/table_test.go`

## 4. errorlint

- [x] 4.1 Add `//nolint:errorlint // <site-specific reason>` at the ten dest sites. Convert none to `errors.Is`. Reasons: `err == errUnreachable` is tcp-session identity (pool wait / not-from-New); the rest are identity on sentinels this package or `ReadSlice` returns exactly. Do not cite Yaegi `errors.As`.

## 5. Catalog and prove

- [x] 5.1 Update `knowledge/devdocs/std_go_test-suites.md`: pin v1.63.4, rejected `testpackage`, omitted goimports/gofumpt, `checkExported` trap
- [x] 5.2 `golangci-lint run` clean with v1.63.4
- [x] 5.3 `go test -short ./...` including Yaegi. Record `localTests`
- [x] 5.4 `openspec validate --change harden-golangci-lint --strict`
