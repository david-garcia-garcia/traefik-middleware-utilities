---
url: https://github.com/traefik/yaegi/blob/v0.16.1/stdlib/unsafe/unsafe.go
title: stdlib/unsafe/unsafe.go
fetched: 2026-09-11
authority: source
ref: github.com/traefik/yaegi@v0.16.1:stdlib/unsafe/unsafe.go
---

`init` sets `Symbols["unsafe/unsafe"]["Add"]`, `Sizeof`, `Alignof`, `Offsetof` (Offsetof is `func(interface{}) uintptr { return 0 }` for signature check only).

`//go:generate ../../internal/cmd/extract/extract unsafe` produces the Pointer-only extract files.
