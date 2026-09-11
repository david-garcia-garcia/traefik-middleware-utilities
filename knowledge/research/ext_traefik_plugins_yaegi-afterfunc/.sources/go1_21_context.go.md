---
url: https://github.com/traefik/yaegi/blob/v0.16.1/stdlib/go1_21_context.go
title: Yaegi v0.16.1 stdlib go1.21 context extract
fetched: 2026-09-11
authority: source
ref: github.com/traefik/yaegi@v0.16.1:stdlib/go1_21_context.go
---

Build: `go1.21 && !go1.22`.

`init` sets `Symbols["context/context"]` including `"AfterFunc": reflect.ValueOf(context.AfterFunc)` next to `Background`, `WithCancel`, `WithTimeout`, `WithoutCancel`.

`stdlib/go1_22_context.go` (build `go1.22`) maps the same `"AfterFunc"` key. This tag has no `go1_23_context.go`.
