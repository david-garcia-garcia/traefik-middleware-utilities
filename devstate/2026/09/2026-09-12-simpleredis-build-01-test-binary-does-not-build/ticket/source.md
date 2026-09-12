# build-01 — The `simpleredis` test binary does not compile in the working tree

- **Axis**: Build / test coverage
- **Severity**: hard (but working tree only — `HEAD` is clean)
- **Where**: `simpleredis/interpretedcost_test.go:21`, `:190`, `:273`, `:380`
- **Status**: not applied

## What I found

`go test ./simpleredis/` fails to build:

```
simpleredis\interpretedcost_test.go:21:2: undefined: writeGopathFile
simpleredis\interpretedcost_test.go:190:2: undefined: writeGopathFile
simpleredis\interpretedcost_test.go:273:2: undefined: writeGopathFile
simpleredis\interpretedcost_test.go:380:2: undefined: writeGopathFile
FAIL	github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis [build failed]
```

Nothing in the package defines `writeGopathFile`. The package has
`writeGopathSimpleredis` and `writeGopathClientprobe` in `yaegi_test.go`, but not
the generic per-package variant these four call sites want. A helper of that name
does exist in `tokenbucket`, `windowcounter` and `reclaim`, so this looks like a
file moved into `simpleredis` without its dependency.

Scope check: `interpretedcost_test.go` and `bench_test.go` are both **untracked**,
so `HEAD` builds and tests fine — confirmed by exporting `HEAD` to a temp directory
and running the suite green. This is uncommitted work in progress, not a shipped
break.

Because a build failure takes down the *whole test binary*, the practical effect
while it persists is that **zero** `simpleredis` tests run — not the 86% statement
coverage recorded in `../simpleredisfixes/README.md`, but none of it. Every audit
in this backlog had to add a local stub before anything could be executed.

## Why it matters

Two distinct risks.

**Right now:** anyone working in this tree gets no test signal from `simpleredis`
at all, and a `go test ./simpleredis/` failure reads as "build failed" rather than
"a helper is missing", so it is easy to assume the package is broken. This is the
state the package is in while the most severe findings in this backlog
([bug-01](bug-01-panic-leaks-pool-token.md),
[bug-02](bug-02-unbounded-reply-allocation.md)) are being fixed — precisely when
test feedback matters most.

**On commit:** `.github/workflows/ci.yml:67` runs `go test ./...`, so committing
these two files as-is turns CI red on every subsequent PR. And because the failure
is a build failure rather than a test failure, it masks everything else in the
package — a real regression introduced at the same time would be invisible.

There is a related pre-existing hazard in the same area: `go test ./...` also fails
on the untracked `apm_modules/` tree
(`found packages pool (idle-lock.go) and geo (label-with-debug.go)`), which is
untracked but **not** git-ignored. Same failure shape, same trigger — someone
commits it — so both should be closed together. See
[ci-01](ci-01-no-race-detector-in-ci.md).

## Expected gain

A buildable test binary, which is the precondition for every other fix in this
backlog being verifiable. Nothing else in this file matters until this one is done.

## How to fix

Define the missing helper in the `simpleredis` package. It is the generic form of
the two helpers already in `yaegi_test.go` — write one source file into a named
package directory under a temp GOPATH:

```go
// writeGopathFile writes src as GOPATH/src/<pkgDir>/<fileName> for interp to import.
func writeGopathFile(t testing.TB, goPath, pkgDir, fileName, src string) {
	t.Helper()
	dir := filepath.Join(goPath, "src", pkgDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}
```

That signature satisfies all four call sites and is what I used to run the audit.

Better, given the helper already exists in three other packages: factor it out
once into a small internal test-support package (or have `writeGopathClientprobe`
delegate to it) rather than adding a fourth copy. The four packages all need the
same "materialise a GOPATH for Yaegi" primitive, and the duplication is what let
this drift in the first place.

Two follow-ons:

- **Add `apm_modules/` to `.gitignore`** so `go test ./...` cannot be broken by
  agent tooling.
- **Commit the two untracked test files together with the helper**, not separately.
  They carry the measurements `../simpleredisfixes/README.md` cites, so the numbers
  in that document are currently unreproducible from a clean checkout.

## How to prove it

`go vet ./...` and `go test -count=1 ./...` from a clean checkout, both green. `vet`
is the faster signal since it catches the build break without running anything.

The durable guard is CI itself: once the files are committed and CI runs
`go test ./...`, this class of break cannot recur silently. Worth confirming the
Yaegi tests actually execute afterwards rather than being skipped — they are the
tests that depend on the restored helper, and `-run Yaegi` with `-v` should show
five passing cases:

```
--- PASS: TestYaegi_NewGetSetDel
--- PASS: TestYaegi_IncrAndEval
--- PASS: TestYaegi_EvalNoScriptFallback
--- PASS: TestYaegi_MSetEXNative
--- PASS: TestYaegi_MSetEXLua
```
