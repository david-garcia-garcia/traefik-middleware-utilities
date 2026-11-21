# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise it is closed and the key is dropped. The caller type-asserts. The caller creates the table with `New(Config)` and holds it.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`; a process-wide table in `reclaim`

**Incarnation**:
One value from `create` to `close`, together with the holders bound to it. A key has at most one at a time. Sleeping and waking do not end an incarnation: a woken value is the same pointer the earlier holder had.
_Avoid_: calling a new value after a reclaim "the same incarnation"; it is not, and its Close hook runs separately

**Open**:
Create-once for a key (`create` takes no args — Yaegi cannot call `func(context.Context) (any, error)`). The caller passes a `*slog.Logger` (required; no table logger and no fallback) and a `Hooks` value stored on the incarnation at put. A later `Open` does not run create, does not replace hooks, and wakes the value first if it was asleep. `ctx` must not be nil. Traefik's `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`. `(value, nil)` means this call bound a holder that was still live at return; a canceled ctx at bind returns `(nil, ctx.Err())` and not the pointer. Nil arguments error as table, then logger, then create.
_Avoid_: Put vs Bind as two public calls; a nil holder context; assuming `Open` returns fast when the value has a slow `Wake`; type-switch discovery on the stored `any`; type-asserting when `err != nil`

**OpenWithHooks**:
`Open` with `create` returning `(any, Hooks, error)` instead of taking hooks beside it. Use this when Sleep, Wake, or Close must act on the value being created: those funcs are built inside `create` and stored at put. A later `Open` binds without running `create` and keeps the stored hooks, including `EnforceCloseBeforeOpen`.
_Avoid_: declaring a variable before `Open` only so hooks can close over it; faking `EnforceCloseBeforeOpen` from outside the package

**OpenTyped**:
Package function `OpenTyped[T any]`: `OpenWithHooks` that returns the stored value as `T`. On a type mismatch it returns the zero `T` and `reclaim: open %q: want %T, got %T`. The instantiation must stay a call expression in a package that can name `T`.
_Avoid_: a package-level var, type alias, or struct field whose type names a generic instantiation from another package (Yaegi v0.16.1); `Table[T]`

**Hooks**:
The optional `Sleep`, `Wake`, and `Close` funcs stored at put, plus `EnforceCloseBeforeOpen`. They arrive beside `create` on `Open`, or from `create` on `OpenWithHooks` and `OpenTyped`. A nil func skips that event. They belong to the incarnation created at put, not to the latest holder. `EnforceCloseBeforeOpen` defaults off: the table unmaps before Close, so a later `Open` of that key may create while Close is still in flight. Set it when the value owns something exclusive that cannot be held twice (an mmap, a file lock, a listening port, a connection). The cost of turning it on is that a slow or blocking Close now delays the next `create` for that key, which on the Traefik path means delaying a config reload. The ending path reads the stored flag, not a later `Open`'s argument.
_Avoid_: optional methods on the stored value; replacing hooks on bind or reclaim; putting this knob on `Table` or `Config` (it is per incarnation, not per table)

**Lifecycle**:
The four events the table drives on one stored value: `create -> (sleep -> wake)* -> sleep -> close`. `create` and `close` run once. `sleep` and `wake` are a matched, repeating pair. `close` is always preceded by `sleep`, on every ending path, so cleanup is never written twice.
_Avoid_: a house `dispose func(any)` on `Open`; cleanup in `Close` that `Sleep` already did

**Sleep**:
Optional `Hooks.Sleep`, called when the last holder's context is Done. The value stays stored and keeps its identity, and releases what is expensive to hold idle. It is not a close: the value must be resumable. If Sleep panics, the table does not park the value asleep: it Closes that incarnation and unmaps the key so a later Open creates. When that incarnation stored `EnforceCloseBeforeOpen`, the key stays mapped until Close returns, so a later Open waits and then creates; when the flag is unset, unmap happens first (a later Open may create during Close).
_Avoid_: releasing something in `Sleep` that `Wake` cannot get back

**Wake**:
Optional `Hooks.Wake`, called when an `Open` finds a stored, sleeping value. `Open` does not return until `Wake` has returned, so a caller never receives a sleeping value. `Wake` has no resume-failure path: a value that cannot guarantee resume simply does not pass Sleep and Wake hooks. If Wake panics, that is a broken hook: `Open` returns an error wrapping the panic, not the pointer, and the incarnation is Closed and unmapped. Concurrent waiters on that Wake receive the same error. When that incarnation stored `EnforceCloseBeforeOpen`, a later Open that arrives while Close is still in flight waits, then creates; waiters already parked on the Wake still receive the wrapping error.
_Avoid_: an error return or a create-fallback on a Wake that returns; work in `Wake` slow enough to stall a Traefik reload

**Close**:
Optional `Hooks.Close`, called once when the incarnation ends (grace elapsed, `Reset`, zero-grace drop, Sleep panic, or Wake panic), always after Sleep. The table emits `reclaim_dispose` after Close returns, or after a recovered Close panic (which also logs `reclaim_hook_panic` at error). By default the key is unmapped first, so a later `Open` may create during Close. When that incarnation stored `EnforceCloseBeforeOpen`, the key stays mapped until Close returns, so a later `Open` waits and then creates. That includes the Sleep-panic and Wake-panic endings.
_Avoid_: cleanup in Close that Sleep already did; a Close hook that blocks; Traefik plugin `Close`

**Grace**:
How long a **sleeping** value is kept before it is closed and its key dropped. An `Open` inside that window is a reclaim: it wakes the stored value instead of creating one. Zero grace keeps nothing — `sleep` and `close` run back to back and there is no window to be woken in. Negative grace is `DefaultGrace` (10s).
_Avoid_: passing `0` when you meant the product default; reading grace as "how long the value stays live" — it is asleep for all of it

## Overview

`reclaim` is reusable across middleware packages. Yaegi panics on `reclaim.Table[*T]` from another package; it loads a non-generic table of `any` and a type-assert in the caller.

Because grace costs a sleeping value rather than a live one, a long grace is cheap.

## How to use

- Production: hold `reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})` at package scope in the plugin and call `OpenTyped` (or `Open` / `OpenWithHooks`). Tests: `New(Config{Grace: short})`. `logger` is required. Nil hook funcs skip that event.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`'s logger; orphan/dispose use the last `Open` on the key.
- `reclaim_hook_panic` is the one error-level line: a hook panicked and was recovered. Alert on it — the table kept running, but that hook is broken.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`. `Open` returns `(nil, ctx.Err())` when that context is already done at bind; do not type-assert a nil pointer.
- Prefer `OpenTyped` so hooks are built inside `create` and the return is already `T`. `Open` still accepts hooks beside `create`. Do not type-switch the stored `any` for Sleep, Wake, or Close.
- Write the Close hook assuming Sleep already ran. Do not block in Close.
- Set `Hooks.EnforceCloseBeforeOpen` when the value owns an exclusive resource that cannot be held twice. Leave it off (the default) when overlap is acceptable.
- Prefix keys when more than one type shares a table.

## Pattern snippet

```go
var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})

stored, err := reclaim.OpenTyped[*BIN](ctx, table, "bin:"+hash, logger, func() (any, reclaim.Hooks, error) {
	v := newBIN(cfg)
	return v, reclaim.Hooks{
		Sleep:                  v.Sleep,
		Wake:                   v.Wake,
		Close:                  v.Close,
		EnforceCloseBeforeOpen: true,
	}, nil
})
if err != nil {
	return nil, err
}
```

## Key files

- `reclaim/table.go` — `Table`, `Config`, `New`, `Open`, `OpenWithHooks`, the slot state machine, logs
- `reclaim/opentyped.go` — `OpenTyped`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`, `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). `New(Config{Grace: 0})` ends the incarnation as soon as the last holder is gone.
- Yaegi: do not write `Table[*T]` on a type from another package, and do not give `create` an argument. `OpenTyped[T]` must stay a call expression in a package that can name `T`. Do not park grace expire in an interpreted `go` + `select` on a timer and a wake channel — Yaegi v0.16.1 `_select` can miss the timer (use `time.AfterFunc`).
- A later `Open` uses the hooks stored at put; its own `Hooks` argument is ignored.
- A second `Open` while the incarnation is live or in grace returns the same value when that `Open`'s context is still live at bind. If `ctx.Err()` is already set, `Open` returns that error and not the pointer; Close may already have run when that call was the last holder.
- Tests assert the `msg` constants. A test that cancels a holder and immediately calls `Open` is usually not testing the wake branch — wait for `reclaim_orphan` first.
- `Open` blocks for as long as `Wake` takes. Keep `Wake` cheap.
- A panicking `create`, Sleep, or Wake unsticks the key (later `Open` can create). AfterFunc recovers Sleep and Close panics so they cannot kill the process.
- A Close hook that blocks blocks the drop or `Reset` goroutine, and also `Open` when a canceled bind is the last holder at zero grace. When `EnforceCloseBeforeOpen` is set it also blocks any `Open` of that key until it returns. Keep Close cheap.
- At zero grace an `Open` that races the orphan log is a plain bind, not a reclaim. `reclaim_dispose` of the previous incarnation precedes `reclaim_put` of the next only when that previous incarnation stored `EnforceCloseBeforeOpen`.
- `Table.Reset` is tests only. It must not race an `Open` on the same key. Reset unmaps first regardless of `EnforceCloseBeforeOpen`. A Sleep panic during Reset skips `reclaim_orphan` and still disposes.
- A holder whose `Done` is nil (`context.Background()`, Yaegi) is still accepted. The table polls `Err` until it is set, or until that incarnation has ended (`Reset`, grace, last-holder close, Sleep-panic, or Wake-panic). The bind-path snapshot of `finished` is taken under the table mutex: if that snapshot is already nil, no watcher is started. When the incarnation has ended the watcher exits without dropping that holder, so a later incarnation of the same key is unchanged. A Sleep-panic or Wake-panic ending that stored `EnforceCloseBeforeOpen` still ends the real incarnation before the closer occupies the key; no holder binds to that closer. Do not pass Background in production — Traefik `New` ctx is `WithCancel`.
- Every lock-held region releases the table mutex with `defer`. A panic recovered at the Yaegi plugin boundary cannot leave the mutex held; a later `Open` on that table still runs. `Open` on a zero-value `Table{}` returns an error (same class as a nil table or nil logger). Construct with `New(Config)`.

## Audited invariants

Ruled out by a race, leak, and coverage audit of `reclaim/table.go`, so the next reader does not
re-derive them:

- Holder accounting is balanced: exactly one `drop` per bind, and `finishBind` either drops or registers a watcher, never both.
- `expire` against a reclaim, `Reset` against `drop`, and `Reset` against `expire` all serialize under `t.mu` and produce no double `Close`. Grace timers are stopped on reclaim and on `Reset`, and every `context.AfterFunc` registration pairs with a live holder.
- `slot.finished` was the only field ever read outside `t.mu`. The bind path snapshots it under the lock and starts no watcher when the snapshot is nil.
- `close(slot.ready)` is balanced on every ending path, so no double-close panic is reachable. Hooks and every `slog` call run outside the mutex, so no caller-supplied hook can wedge the table.

Two internal shapes are unreachable from the table and are pinned by tests rather than deleted:
`claimDrop`'s `slotBusy` park (a busy slot only ever has holders that have not bound yet) and
`expire`'s early return for an incarnation another path already claimed. Both are guards whose
alternative is acting on a slot the goroutine does not own. `waitCtx`'s Done-channel branch was
deleted instead, because `dropWhenDone` hands every holder that has a Done channel to
`context.AfterFunc`, so no caller could reach it.

`reclaim/table_gaps_test.go` reaches those shapes by taking `tab.mu` and writing `slot` state
directly, as `mustSlot` and `readState` do. A locking rewrite that renames `Table.mu`, `slot.state`,
`slot.holders`, `slot.ready`, or the `slotState` constants must update that file in the same change.

Reproduce the audit with `go test -count=1 ./reclaim/` plus, for the race detector,
`docker run --rm -v "<repo>:/src" -w /src golang:1.25 go test -race -count=1 ./reclaim/`.
