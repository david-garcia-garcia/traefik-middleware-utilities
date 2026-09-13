# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise it is closed and the key is dropped. The caller type-asserts.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`

**Default**:
The process-wide table (`reclaim.Default`, `reclaim.Open`). One incarnation per key for the whole process.
_Avoid_: one `NewTable` per caller when they should share; unprefixed keys that can collide

**Incarnation**:
One value from `create` to `close`, together with the holders bound to it. A key has at most one at a time. Sleeping and waking do not end an incarnation: a woken value is the same pointer the earlier holder had.
_Avoid_: calling a new value after a reclaim "the same incarnation"; it is not, and its Close hook runs separately

**Open**:
Create-once for a key (`create` takes no args — Yaegi cannot call `func(context.Context) (any, error)`). The caller passes a `*slog.Logger` (required; no table logger and no fallback) and a `Hooks` value stored on the incarnation at put. A later `Open` does not run create, does not replace hooks, and wakes the value first if it was asleep. `ctx` must not be nil. Traefik's `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`.
_Avoid_: Put vs Bind as two public calls; a nil holder context; assuming `Open` returns fast when the value has a slow `Wake`; type-switch discovery on the stored `any`

**Hooks**:
The optional `Sleep`, `Wake`, and `Close` funcs passed to `Open`. A nil field skips that event. They belong to the incarnation created at put, not to the latest holder.
_Avoid_: optional methods on the stored value; replacing hooks on bind or reclaim

**Lifecycle**:
The four events the table drives on one stored value: `create -> (sleep -> wake)* -> sleep -> close`. `create` and `close` run once. `sleep` and `wake` are a matched, repeating pair. `close` is always preceded by `sleep`, on every ending path, so cleanup is never written twice.
_Avoid_: a house `dispose func(any)` on `Open`; cleanup in `Close` that `Sleep` already did

**Sleep**:
Optional `Hooks.Sleep`, called when the last holder's context is Done. The value stays stored and keeps its identity, and releases what is expensive to hold idle. It is not a close: the value must be resumable. If Sleep panics, the table does not park the value asleep: it Closes that incarnation and unmaps the key so a later Open creates.
_Avoid_: releasing something in `Sleep` that `Wake` cannot get back

**Wake**:
Optional `Hooks.Wake`, called when an `Open` finds a stored, sleeping value. `Open` does not return until `Wake` has returned, so a caller never receives a sleeping value. `Wake` has no resume-failure path: a value that cannot guarantee resume simply does not pass Sleep and Wake hooks. If Wake panics, that is a broken hook: `Open` returns an error wrapping the panic, not the pointer, and the incarnation is Closed and unmapped.
_Avoid_: an error return or a create-fallback on a Wake that returns; work in `Wake` slow enough to stall a Traefik reload

**Close**:
Optional `Hooks.Close`, called once when the incarnation ends (grace elapsed, `Reset`, or zero-grace drop), always after Sleep. The table waits until it has returned before `reclaim_dispose`.
_Avoid_: cleanup in Close that Sleep already did; a Close hook that blocks; Traefik plugin `Close`

**Grace**:
How long a **sleeping** value is kept before it is closed and its key dropped. An `Open` inside that window is a reclaim: it wakes the stored value instead of creating one. Zero grace keeps nothing — `sleep` and `close` run back to back and there is no window to be woken in. Negative grace is `DefaultGrace` (10s).
_Avoid_: passing `0` when you meant the product default; reading grace as "how long the value stays live" — it is asleep for all of it

## Overview

`reclaim` is reusable across middleware packages. Yaegi panics on `reclaim.Table[*T]` from another package; it loads a non-generic table of `any` and a type-assert in the caller.

Because grace costs a sleeping value rather than a live one, a long grace is cheap.

## How to use

- Production: `reclaim.Open(ctx, key, logger, create, hooks)` (process table). Tests: `NewTable` with a short grace, or `Reset`. `logger` is required. Nil hook funcs skip that event.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`'s logger; orphan/dispose use the last `Open` on the key.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Pass `Hooks` that close over a pointer assigned inside `create`. Do not type-switch the stored `any` for Sleep, Wake, or Close.
- Write the Close hook assuming Sleep already ran. Do not block in Close.
- Prefix keys when more than one type shares Default.

## Pattern snippet

```go
var v *BIN
stored, err := reclaim.Open(ctx, "bin:"+hash, logger, func() (any, error) {
	v = newBIN(cfg)
	return v, nil
}, reclaim.Hooks{
	Sleep: func() { v.Sleep() },
	Wake:  func() { v.Wake() },
	Close: func() { v.Close() },
})
w := stored.(*BIN)
```

## Key files

- `reclaim/table.go` — `Table`, `Open`, the slot state machine, logs
- `reclaim/default.go` — `Default`, package `Open`, `Reset`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`, `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). `NewTable(0)` ends the incarnation as soon as the last holder is gone.
- Yaegi: do not write `Table[*T]` on a type from another package, and do not give `create` an argument.
- A later `Open` uses the hooks stored at put; its own `Hooks` argument is ignored.
- A second `Open` while the incarnation is live or in grace returns the same value.
- Tests assert the `msg` constants. A test that cancels a holder and immediately calls `Open` is usually not testing the wake branch — wait for `reclaim_orphan` first.
- `Open` blocks for as long as `Wake` takes. Keep `Wake` cheap.
- A panicking `create`, Sleep, or Wake unsticks the key (later `Open` can create). AfterFunc recovers Sleep and Close panics so they cannot kill the process.
- A Close hook that blocks blocks the drop or `Reset` goroutine. Keep Close cheap.
- At zero grace an `Open` that races the orphan log is a plain bind, not a reclaim.
- `Reset` is tests only. It must not race an `Open` on the same key.
