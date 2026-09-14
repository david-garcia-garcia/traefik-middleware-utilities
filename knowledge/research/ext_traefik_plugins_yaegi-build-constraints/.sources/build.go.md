---
url: https://github.com/traefik/yaegi/blob/v0.16.1/interp/build.go
title: interp/build.go
fetched: 2026-09-13
authority: source
ref: github.com/traefik/yaegi@v0.16.1:interp/build.go
---

`buildLineOk(ctx, line)` short-circuits on anything that is not the legacy form:

```go
if len(line) < 7 || line[:7] != "+build " {
	return true
}
```

`true` means keep the file. `buildOptionOk` (comma AND) and `buildTagOk` (`!` negation, GOOS/GOARCH/tag match) are called only from the loop below that guard, so they are unreachable for `//go:build` lines.

`skipFile(ctx, p, skipTest)` implements the filename mechanism and works:

- returns `true` for paths not ending `.go`, base names starting `_` or `.`, and `_test` when `skipTest`
- splits the base name on `_`; with two or more trailing tokens it switches on `(a[last-1], a[last])` against `ctx.GOOS` / `knownArch` / `ctx.GOARCH`
- with a single trailing token: `if x := a[last]; knownOs[x] && x != ctx.GOOS || knownArch[x] && x != ctx.GOARCH { return true }`

`knownOs` and `knownArch` are package-level maps in the same file (`aix`, `android`, `darwin`, `dragonfly`, `freebsd`, `illumos`, `ios`, `js`, `linux`, `windows`, … ).

Net effect: OS/arch selection by **filename** is honored; constraint **comments** in modern syntax are not.
