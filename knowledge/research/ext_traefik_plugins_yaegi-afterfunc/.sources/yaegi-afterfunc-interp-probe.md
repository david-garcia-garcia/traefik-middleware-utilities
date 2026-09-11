---
url: %TEMP%/yaegi-afterfunc-interp-probe
title: Throwaway Yaegi GOPATH interp of context.AfterFunc
fetched: 2026-09-11
authority: source
---

Throwaway GOPATH interp, not a committed file. Module lived at `%TEMP%/yaegi-afterfunc-interp-probe` (`github.com/traefik/yaegi v0.16.1` in that `go.mod` / `go.sum`) and was deleted after the run. Nested GOPATH for the interpreted package was a second OS temp dir. Measured 2026-09-11 on `go1.25.6 windows/amd64`.

Interpreter: `github.com/traefik/yaegi v0.16.1`, `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)` only (no `unsafe`; matches this repo `docker-compose.yml` `useunsafe: false`). Shape copied from this repo `reclaim/yaegi_test.go` `evalHookprobe` (not edited; not `go test` of the product). Interpreted package `afterprobe` under `GOPATH/src/afterprobe`.

Symbol Eval (same interp, `import "context"` then `Eval("context.AfterFunc")`):

- `eval-import-context: ok`
- `eval-symbol-AfterFunc: ok kind=func type=func(context.Context, func()) func() bool`

Load/run of interpreted `afterprobe.RunAfterFunc()` (fresh interp):

- `context.WithCancel(context.Background())`
- `context.AfterFunc(ctx, f)` where `f` is `func() { n.Add(1) }` (`sync/atomic`)
- `cancel()`
- wait up to 2s for `n != 0`

Result: `eval-import-afterprobe: ok` then `eval-call-AfterFunc: ok result=ran=1`. No interp/eval error. The callback ran once after cancel.
