# Requirement
IssueKey: 2026-09-11-reclaim-afterfunc

## Problem

Every successful `Open` parks one goroutine for that holder until the context is done. Traefik's real path (one Open per plugin instance per reload) makes this cheap, but many Opens per request would pay a parked goroutine each. The ticket wants `context.AfterFunc` for holders whose `Done()` is non-nil, after a Yaegi-compat spike, and to keep the poll path when `Done()` is nil.

## Current (code)

- `reclaim/table.go:187`, `reclaim/table.go:240`, `reclaim/table.go:269` — put, bind, and reclaim each `go t.watch(key, incarnation, ctx)` after a successful bind.
- `reclaim/table.go:273-277` — `watch` waits then `drop`. Ticket cited 274:277 as the spawn; that is the wait function, not the `go`.
- `reclaim/table.go:114-123` — `waitCtx` already branches: non-nil `Done()` blocks on `<-done`; nil `Done()` (Background / Yaegi-shaped) polls `Err()` every 20ms. It does not only `<-ctx.Done()`.
- `reclaim/table_test.go:294-317` — `nilDoneCtx` (`Done()` returns nil; `cancel` sets `Err()`). Lines 296:311 in the ticket are the type and methods.
- `reclaim/table_test.go:1181-1198` — `TestTable_HolderWithoutDoneChannelIsPolled` proves the poll path; cancel is `Err()` not a closed `Done()`.
- `reclaim/table_test.go:1172-1179` — `TestTable_OpenBackgroundDoesNotPanic` binds `context.Background()` (nil `Done()`, never `Err()`).
- `reclaim/default.go:25-28` — package `Open` forwards to `Default().Open` (same watch spawn).
- `go.mod:3` — `go 1.21` (language `AfterFunc` is in this pin).
- `go.mod:5` — `github.com/traefik/yaegi v0.16.1`.
- `openspec/specs/std_go_reclaim_context-lease/spec.md:26-27` — nil-`Done` holder stays live until `ctx.Err()` is set.
- `openspec/specs/std_go_reclaim_context-lease/spec.md:172-188` — every goroutine the table starts for a key SHALL exit once holders are Done and the incarnation has ended.
- `knowledge/devdocs/std_go_reclaim.md:18` — Traefik `New` ctx is `WithCancel`; next reload cancels it before the next `New`.
- `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/notes.md` — Yaegi v0.16.1 `stdlib` maps `context.AfterFunc` in `go1_21_context.go` and `go1_22_context.go`. Interp-time call from interpreted `reclaim` is **not measured**.
- `context.AfterFunc` in this tree: **not found** (no call sites).

## Desired

- Prototype replacing the parked `watch` goroutine with `context.AfterFunc` when `ctx.Done() != nil` (Traefik `WithCancel` / timeout holders).
- Keep today's goroutine+poll fallback when `Done()` is nil (`nilDoneCtx`, `context.Background()`). AfterFunc never runs `f` if `Done()` is nil.
- Confirm Yaegi v0.16.1 can interpret a `reclaim` that calls `context.AfterFunc` before adopting (ticket: not worth doing blind).
- Preserve drop/lifecycle and the goroutine-exit spec: AfterFunc still runs `f` in a goroutine at cancel time; that goroutine must exit like `watch` does.

## Affected

- `reclaim/table.go` (`waitCtx` / `watch` / the three `go t.watch` sites)
- `reclaim/table_test.go` (poll test must stay; AfterFunc path needs compiled coverage)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` if the wait mechanism is specified beyond "holders go Done"
- Yaegi interp / Pester e2e if the spike says the interpreted package must load `AfterFunc`

## Out of scope

- Adopting AfterFunc without the Yaegi spike
- Upgrading Traefik or the Yaegi pin
- Changing grace, hooks, create signature, or logging
- New API to open many keys per request (that is the cost story, not an ask)
- Replacing the nil-`Done` poll with AfterFunc

## Unknowns

- Whether interpreted `reclaim` (Traefik local plugin GOPATH) can call `context.AfterFunc` end-to-end. The symbol is in Yaegi v0.16.1's extract; that is not a load/run proof.
- Which Go build tag Traefik v3.7.11's Yaegi uses (`go1.21` vs `go1.22` extracts; both list AfterFunc). No `go1_23_context.go` in that Yaegi tag.
- Whether AfterFunc's stop func is needed on `Reset` / stale-incarnation drop, or registering `drop` is enough.
- Ticket calls this worth prototyping, not a must-land; explore decides ship vs note.

## Tensions

- Ticket cites `274:277` as the `go t.watch` spawn; those lines are `watch` itself. Spawns are 187 / 240 / 269.
- Ticket says watch "just blocks on `<-ctx.Done()`"; `waitCtx` already special-cases nil `Done()` and polls `Err()`.
- Ticket says AfterFunc registers "without spawning a goroutine at all". `AfterFunc` still runs `f` in a goroutine once the context is done; it avoids a **parked waiter** for the hold lifetime, not every goroutine.
- Ticket says Yaegi "would need" AfterFunc in the symbol table; the pinned extract already has it. The remaining catch is interp behavior, not the map key.
- Ticket is explicitly "worth prototyping, not worth doing blind" — desired is a gated prototype, not an unmeasured replace.
