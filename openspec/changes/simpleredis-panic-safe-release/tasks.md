## 1. Product

- [x] 1.1 Move `exec`'s `do` + `release` into `runOnConn` with deferred `release`, `reusable` declared false before the defer, and `values, reusable, err = sr.do(...)`
- [x] 1.2 Keep dest `contextStop` after `do` inside `runOnConn` (`reusable = false` then return)
- [x] 1.3 Add `handedOff` on `borrow` with deferred `freeInUseTurn()` when not handed off; remove the two explicit error-path `freeInUseTurn()` calls
- [x] 1.4 Update the stale panic-loses-turn comment on `do` in `resp.go`

## 2. Tests

- [x] 2.1 Add `simpleredis/panic_safety_test.go` `TestPanicInDoReturnsTurnAndClosesSocket` (panicking `io.Writer`, no `zz_` / proto / scratch)
- [x] 2.2 Add `simpleredis/yaegi_defer_test.go` `TestYaegi_DeferRunsOnPanic` as a permanent probe (explicit panic, interpreter `errors.As`, nil-map write) with a comment that names why it exists
- [x] 2.3 `go vet ./simpleredis/`
- [x] 2.4 `go test -count=1 -timeout 300s ./simpleredis/` — no pre-existing test edits
- [x] 2.5 `go test -count=5 -timeout 600s -run "Pool|Panic|Release|Borrow|Turn" ./simpleredis/`

## 3. Spec and usage

- [x] 3.1 Add the Yaegi-defer-runs gotcha on `knowledge/devdocs/std_go_simpleredis.md`
- [x] 3.2 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests
- [x] 3.3 Run `openspec validate --change simpleredis-panic-safe-release --strict`
