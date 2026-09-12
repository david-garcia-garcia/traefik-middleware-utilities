# Yaegi unsafe symbols

Which `unsafe` package symbols Yaegi v0.16.1 exports when Traefik registers `stdlib/unsafe`. This module and Traefik v3.7.11 both pin `github.com/traefik/yaegi v0.16.1`.

## Exported symbols

Hand-written `stdlib/unsafe/unsafe.go` registers functions on `Symbols["unsafe/unsafe"]`:

- `Add`
- `Sizeof`
- `Alignof`
- `Offsetof` (stub: signature check only, always returns 0)

Generated extracts register **only** the `Pointer` type:

- `stdlib/unsafe/go1_21_unsafe.go` (`go1.21 && !go1.22`)
- `stdlib/unsafe/go1_22_unsafe.go` (`go1.22` — no upper bound; Go 1.23+ builds use this file)

There is no `go1_23_unsafe.go`. Combined surface: `Pointer`, `Add`, `Sizeof`, `Alignof`, `Offsetof`.

**Absent** from every file in that directory: `Slice`, `String`, `StringData`, `SliceData` (Go 1.20+ stdlib `unsafe` helpers that go-redis v9 uses). Interpreted code that names those symbols fails at import/eval time (`package unsafe "unsafe" has no symbol Slice`), including when Traefik has registered `unsafe.Symbols` and even with interp `Unrestricted`.

Legacy pointer-cast tricks (`*(*string)(unsafe.Pointer(&b))` and the string-header struct for `[]byte`) need those registered symbols (`Pointer`) plus the operator/manifest dual gate documented in `ext_traefik_plugins_useunsafe/`. They are not available under `stdlib.Symbols` alone.

Source: [yaegi@v0.16.1 stdlib/unsafe](https://github.com/traefik/yaegi/tree/v0.16.1/stdlib/unsafe); extracts [.sources/unsafe.go.md](.sources/unsafe.go.md), [.sources/go1_21_unsafe.go.md](.sources/go1_21_unsafe.go.md), [.sources/go1_22_unsafe.go.md](.sources/go1_22_unsafe.go.md). Pin: this repo `go.mod` `require github.com/traefik/yaegi v0.16.1`.

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Add, Sizeof, Alignof, Offsetof registered | yaegi@v0.16.1 stdlib/unsafe/unsafe.go | source |
| Pointer type only in go1.21/go1.22 extracts | yaegi@v0.16.1 go1_21_unsafe.go, go1_22_unsafe.go | source |
| Slice/String/StringData/SliceData not in those maps | same directory listing | source |
