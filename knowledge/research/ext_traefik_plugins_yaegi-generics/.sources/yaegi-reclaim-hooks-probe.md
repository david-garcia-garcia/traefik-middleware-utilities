---
url: %TEMP%/yaegi-reclaim-hooks-probe
title: Throwaway Yaegi GOPATH interp of reclaim.Open and func() hooks
fetched: 2026-09-11
authority: source
---

Throwaway GOPATH interp, not a committed file. Module lived at `%TEMP%/yaegi-reclaim-hooks-probe` and was deleted after the run. Measured 2026-09-11.

Interpreter: `github.com/traefik/yaegi v0.16.1`, `interp.New(Options{GoPath: plugins-local})`, `Use(stdlib.Symbols)` only (no `unsafe`; matches this repo `docker-compose.yml` `useunsafe: false`). GOPATH junction of the worktree at `plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`. Interpreted package `hookprobe` imported this repo's `reclaim` and a throwaway `hooklib`.

Create return through `any` (same package `sleeper` with `Sleep()`):

- `create := func() (any, error) { return &life{}, nil }` then type-switch to `sleeper`: `switch-sleeper=no`
- `%T` is `*struct { Xsleeps atomic.Int32; Xwakes atomic.Int32; Xcloses atomic.Int32 }`
- `reflect` method names empty
- comma-ok `value.(sleeper)`: `ok=false` (no panic on this return-through-any path)
- concrete `value.(*life)`: `ok=true` but `%T` still the synthesized struct

Same interpreted `life` passed into `func(any)` (not through create return): type-switch to `sleeper` ran; no panic.

Stored `func()` values all fired (`sleeps=1 wakes=1 closes=1`):

- method values `value.Sleep`
- same-package `hooks` struct of funcs
- extra Open-like `func()` parameters
- cross-package `hooklib.Hooks{Sleep: value.Sleep, ...}` plus `hooklib.DriveFuncs(...)`

Create signatures that return hooks alongside `any`:

- same-package `func() (any, hooks, error)`: worked
- `hooklib.CreateWithHooks() (any, Hooks, error)` plus `DriveCreate(func() (any, hooklib.Hooks, error))` across packages: worked

This repo `reclaim.NewTable(20ms)` + `tab.Open` from interpreted `hookprobe`, create returning `*life` with Sleep/Wake/Close:

- Traefik-shaped logs printed: `reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_dispose`
- `life.sleeps=0 wakes=0 closes=0`
- Table lifecycle logs run; optional methods on the stored value do not. This is this repo's `reclaim/table.go` `sleepValue` / `wakeValue` / `closeValue` type-switches under Yaegi.

Contrast (not a Yaegi fact): compiled `go test ./reclaim/` in the worktree passed in 1.075s.
