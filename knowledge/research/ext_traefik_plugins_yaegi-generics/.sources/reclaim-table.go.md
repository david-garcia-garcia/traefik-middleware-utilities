---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/c004ceb8d308cfbf6aaaa5957d63a4e42bd3e649/reclaim/table.go
title: reclaim.Open optional lifecycle type-switches
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-middleware-utilities@c004ceb8d308cfbf6aaaa5957d63a4e42bd3e649:reclaim/table.go
---

`Open` create shape: `create func() (any, error)`. Comment: Yaegi cannot call `func(context.Context) (any, error)`.

Optional lifecycle is discovered only by type-switch on the stored `any`:

- `sleepValue` → `case sleeper: typed.Sleep()`
- `wakeValue` → `case waker: typed.Wake()`
- `closeValue` → `case closer: typed.Close()`

Comment at those lookups: comma-ok `value.(sleeper)` panics under Yaegi for a value that reached `any` by being passed in; type-switch reports no match. A value returned through interpreted `func() (any, error)` is a synthesized struct with no methods, so none of the three matches and the optional lifecycle is inert.

Log messages the table always emits (independent of those switches): `reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_dispose` (`MsgPut` / `MsgBind` / `MsgOrphan` / `MsgDispose`).
