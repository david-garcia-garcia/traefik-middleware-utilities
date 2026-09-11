# Yaegi stdlib `context.AfterFunc`

Whether Yaegi v0.16.1 (Traefik v3.7.11 pin, this module's `go.mod`) exports `context.AfterFunc` in `stdlib.Symbols`, so interpreted `reclaim` can name it.

## Symbol is present

Yaegi v0.16.1's generated context extract maps `"AfterFunc"` to `context.AfterFunc` for both Go 1.21 and Go 1.22 builds:

- `stdlib/go1_21_context.go` (`//go:build go1.21 && !go1.22`) — `Symbols["context/context"]["AfterFunc"]`
- `stdlib/go1_22_context.go` (`//go:build go1.22`) — same key

This tag has no `go1_23_context.go`. Traefik v3.7.11 requires `github.com/traefik/yaegi v0.16.1` (same pin as this repo).

Source: `github.com/traefik/yaegi@v0.16.1` `stdlib/go1_21_context.go`, `stdlib/go1_22_context.go`; extract `.sources/go1_21_context.go.md`. This repo `go.mod` require; extract `.sources/go.mod.md`. Traefik pin: `ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`.

## Nil `Done()` never fires

Go `AfterFunc` does not call `f` when `ctx.Done()` is nil (`context.Background`, and this repo's `nilDoneCtx`). A holder that only sets `Err()` must keep a poll (or equivalent). AfterFunc still runs `f` in its own goroutine once a cancellable context is done — it avoids a parked waiter, not the cancel-time goroutine.

Source: https://pkg.go.dev/context@go1.21.13#AfterFunc ; extract `.sources/afterfunc.md`.

## Interp call is not measured

Presence in `stdlib.Symbols` is not a proof that interpreted `reclaim/table.go` calling `context.AfterFunc` loads and runs under Traefik's GOPATH interpreter (`Use(stdlib.Symbols)`, `useunsafe: false`). That spike is still open.

Loader flags: `ext_traefik_plugins_yaegi-generics/.sources/docker-compose.yml.md`.
