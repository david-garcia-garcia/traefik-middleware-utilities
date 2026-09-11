---
url: https://pkg.go.dev/context@go1.21.13#AfterFunc
title: context.AfterFunc (Go 1.21)
fetched: 2026-09-11
authority: official
---

`func AfterFunc(ctx Context, f func()) (stop func() bool)`

Arranges to call `f` in its own goroutine after `ctx` is done. If `ctx` is already done, calls `f` immediately in its own goroutine.

If `ctx.Done()` is nil, `f` is never called.

Returned `stop` stops the association; it does not wait for `f` to finish if `f` has started.
