# Yaegi stdlib `context.AfterFunc`

Whether Yaegi v0.16.1 (Traefik v3.7.11 pin, this module's `go.mod`) exports `context.AfterFunc` in `stdlib.Symbols`, and whether interpreted code can call it.

## Symbol is present

Yaegi v0.16.1's generated context extract maps `"AfterFunc"` to `context.AfterFunc` for both Go 1.21 and Go 1.22 builds:

- `stdlib/go1_21_context.go` (`//go:build go1.21 && !go1.22`) — `Symbols["context/context"]["AfterFunc"]`
- `stdlib/go1_22_context.go` (`//go:build go1.22`) — same key

This tag has no `go1_23_context.go`. Traefik v3.7.11 requires `github.com/traefik/yaegi v0.16.1` (same pin as this repo).

Source: `github.com/traefik/yaegi@v0.16.1` `stdlib/go1_21_context.go`, `stdlib/go1_22_context.go`; extract `.sources/go1_21_context.go.md`. This repo `go.mod` require; extract `.sources/go.mod.md`. Traefik pin: `ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`.

## Nil `Done()` never fires

Go `AfterFunc` does not call `f` when `ctx.Done()` is nil (`context.Background`, and this repo's `nilDoneCtx`). A holder that only sets `Err()` must keep a poll (or equivalent). AfterFunc still runs `f` in its own goroutine once a cancellable context is done — it avoids a parked waiter, not the cancel-time goroutine.

Source: https://pkg.go.dev/context@go1.21.13#AfterFunc ; extract `.sources/afterfunc.md`.

## Interp call runs

Interpreted code can **call** `context.AfterFunc` under Traefik's GOPATH interpreter. Throwaway probe (deleted after the run; not `go test` of this product): `go1.25.6`, `github.com/traefik/yaegi v0.16.1`, `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)` only (`useunsafe` false). Shape copied from this repo `reclaim/yaegi_test.go` `evalHookprobe`.

Eval of the symbol itself (`import "context"` then `Eval("context.AfterFunc")`) succeeded: `kind=func type=func(context.Context, func()) func() bool`. No eval error.

Load/run of interpreted `afterprobe.RunAfterFunc()`: `WithCancel(Background())`, register `AfterFunc(ctx, f)` that increments an atomic, cancel, wait. Result `ran=1` (callback ran). No interp/eval error.

Source: extract `.sources/yaegi-afterfunc-interp-probe.md` (`%TEMP%/yaegi-afterfunc-interp-probe`). Loader flags: `ext_traefik_plugins_yaegi-generics/.sources/docker-compose.yml.md`.
