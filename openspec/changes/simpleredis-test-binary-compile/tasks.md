## 1. Verify dest compile

- [ ] 1.1 Run `go test -c -o NUL ./simpleredis/` on this worktree
- [ ] 1.2 If compile fails because `writeGopathFile` is undefined, add that helper in `simpleredis/yaegi_test.go` with signature `func writeGopathFile(t testing.TB, goPath, pkg, name, src string)` writing `GOPATH/src/<pkg>/<name>`. Do not edit `simpleredis.go`. Do not replace dest test files with caller copies
- [ ] 1.3 If compile already succeeds, leave `yaegi_test.go`, `interpretedcost_test.go`, and `bench_test.go` unchanged

## 2. Prove interpreter tests still run

- [ ] 2.1 Run `go test -short -count=1 -timeout 3m -run ^TestYaegi_ ./simpleredis/` until Init Get Set Del, Incr and Eval, Eval NOSCRIPT, MSetEX native, and MSetEX Lua pass
- [ ] 2.2 Run `openspec validate simpleredis-test-binary-compile --type change --strict`
