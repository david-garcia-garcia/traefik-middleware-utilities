# Yaegi build constraints

Which build-constraint mechanisms Yaegi v0.16.1 honors when it assembles an interpreted package, and which it ignores without a diagnostic. This module and Traefik v3.7.11 both pin `github.com/traefik/yaegi v0.16.1`. Relevant to any Yaegi-loaded package that would otherwise vary behavior per OS.

## `//go:build` lines are never evaluated

`buildLineOk` recognizes only the legacy prefix. Any other comment line is kept:

```go
func buildLineOk(ctx *build.Context, line string) (ok bool) {
	if len(line) < 7 || line[:7] != "+build " {
		return true
	}
	// ... evaluates the OR of space-separated options
}
```

A modern `//go:build linux` line does not start with `+build `, so it returns `true` (keep) and never reaches `buildOptionOk` / `buildTagOk`. Those tag evaluators are reachable **only** from the legacy `// +build` form. Go 1.17 made `//go:build` the canonical form and its tooling maintains it, so a current module cannot practically express a constraint Yaegi will read from a comment.

The failure is silent. Every variant file is parsed into the same interpreted package, nothing warns, and the last-parsed declaration wins — so **filename sort order** decides which body is live. ([yaegi@v0.16.1:interp/build.go](https://github.com/traefik/yaegi/blob/v0.16.1/interp/build.go), [.sources/build.go.md](.sources/build.go.md))

Measured on Linux, yaegi v0.16.1, with the file that should be **excluded** sorting last:

| package | constraint syntax | result on Linux | correct |
| --- | --- | --- | --- |
| `tagmodern` | `//go:build linux` / `//go:build !linux` | `not-linux-file` | no |
| `taglegacy` | `// +build linux` / `// +build !linux` | `linux-file` | yes |

## Filename GOOS/GOARCH suffixes **are** honored

`skipFile` reads the constraint off the base filename, and this path is correct. A trailing pair is matched against both `GOOS` and `GOARCH`; a single trailing token is matched against whichever it names:

```go
if x := a[last]; knownOs[x] && x != ctx.GOOS || knownArch[x] && x != ctx.GOARCH {
	return true
}
```

So `conncheck_linux.go` is skipped when `GOOS != linux` and kept on Linux, and `conncheck_linux_amd64.go` is skipped unless both match. `skipFile` also drops non-`.go` paths, names beginning `_` or `.`, and (when `skipTest`) `_test.go`. ([yaegi@v0.16.1:interp/build.go](https://github.com/traefik/yaegi/blob/v0.16.1/interp/build.go), [.sources/build.go.md](.sources/build.go.md))

**Filename suffixes are therefore the only reliable way to vary an interpreted package by OS under this pin.** In-file constraint comments are not.

## Why go-redis's socket liveness probe loses

go-redis names its files `conn_check.go` (constrained to Unix by a `//go:build` line) and `conn_check_dummy.go` (the no-op). Neither filename carries a GOOS suffix, so `skipFile` keeps both and the comment constraints are ignored. `conn_check_dummy.go` sorts second, so **the no-op wins on Linux**: an interpreted go-redis would silently have no liveness probe. Measured with go-redis's real filenames and constraints, the Linux result was `dummy-noop`.

That probe is only reachable at all when `useUnsafe` registers `syscall` symbols (`ext_traefik_plugins_useunsafe/`), and go-redis does not run under Yaegi regardless (`ext_go-redis_yaegi-compatibility/`). Recorded here because the hazard is general to any large modern library under this pin, not specific to go-redis.

Why this product does not copy that probe, and the rule for this repo: `knowledge/devdocs/std_go_simpleredis.md` (Gotchas).

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| `buildLineOk` keeps any line not prefixed `+build `, so `//go:build` is never evaluated | yaegi@v0.16.1 interp/build.go | source |
| Tag evaluation is reachable only from the legacy `// +build` form | yaegi@v0.16.1 interp/build.go | source |
| `skipFile` honors single and paired GOOS/GOARCH filename suffixes | yaegi@v0.16.1 interp/build.go | source |
| `//go:build` is the canonical form maintained by Go 1.17+ tooling | [cmd/go build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints) | official |
| Both variants load with no diagnostic; filename sort order picks the live body | measured against yaegi v0.16.1 (`tagmodern`, `taglegacy`, go-redis filenames) | inference |
